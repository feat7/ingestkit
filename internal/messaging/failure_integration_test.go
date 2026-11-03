//go:build integration

package messaging

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func integrationClient(t *testing.T) *kgo.Client {
	c, err := kgo.NewClient(kgo.SeedBrokers(brokerAddr(t)))
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}
func createTestTopic(t *testing.T, c *kgo.Client, name string) {
	t.Helper()
	ctx, end := context.WithTimeout(context.Background(), 10*time.Second)
	defer end()
	v := "false"
	req := kmsg.NewPtrCreateTopicsRequest()
	req.Topics = []kmsg.CreateTopicsRequestTopic{{Topic: name, NumPartitions: 1, ReplicationFactor: 1, Configs: []kmsg.CreateTopicsRequestTopicConfig{{Name: "write.caching", Value: &v}}}}
	res, err := c.Request(ctx, req)
	require.NoError(t, err)
	for _, topic := range res.(*kmsg.CreateTopicsResponse).Topics {
		require.Zero(t, topic.ErrorCode)
	}
}
func integrationPool(t *testing.T) *pgxpool.Pool {
	p, err := pgxpool.New(context.Background(), os.Getenv("INGESTKIT_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(p.Close)
	return p
}
func committedOffset(ctx context.Context, c *kgo.Client, group, topic string) (int64, error) {
	req := kmsg.NewPtrOffsetFetchRequest()
	req.Group = group
	req.Topics = []kmsg.OffsetFetchRequestTopic{{Topic: topic, Partitions: []int32{0}}}
	g := kmsg.NewOffsetFetchRequestGroup()
	g.Group = group
	g.Topics = []kmsg.OffsetFetchRequestGroupTopic{{Topic: topic, Partitions: []int32{0}}}
	req.Groups = []kmsg.OffsetFetchRequestGroup{g}
	res, err := c.Request(ctx, req)
	if err != nil {
		return -1, err
	}
	r := res.(*kmsg.OffsetFetchResponse)
	for _, g := range r.Groups {
		for _, t := range g.Topics {
			for _, p := range t.Partitions {
				if p.ErrorCode == 0 {
					return p.Offset, nil
				}
			}
		}
	}
	for _, t := range r.Topics {
		for _, p := range t.Partitions {
			if p.ErrorCode == 0 {
				return p.Offset, nil
			}
		}
	}
	return -1, errors.New("no committed offset")
}
func TestCrashBetweenDatabaseAndOffsetCommit(t *testing.T) {
	if os.Getenv("INGESTKIT_CRASH_HELPER") == "1" {
		pool := integrationPool(t)
		store := persistence.NewEventStore(pool)
		r, err := NewReader(ReaderConfig{Brokers: []string{brokerAddr(t)}, Topic: os.Getenv("CRASH_TOPIC"), DLQTopic: "ingestkit.dlq", GroupID: os.Getenv("CRASH_GROUP")}, storeFunc(func(c context.Context, e []event.Envelope) (persistence.WriteResult, error) {
			result, err := store.WriteBatch(c, e)
			if err == nil {
				fmt.Println("DB_COMMITTED")
				select {}
			}
			return result, err
		}), nil)
		require.NoError(t, err)
		_ = r.Run(context.Background())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	client := integrationClient(t)
	topic, group := "crash-"+uuid.NewString(), "crash-"+uuid.NewString()
	createTestTopic(t, client, topic)
	e := publisherEvent()
	raw, _ := event.EncodeEnvelope(e)
	require.NoError(t, client.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: raw}).FirstErr())
	child := exec.Command(os.Args[0], "-test.run=^TestCrashBetweenDatabaseAndOffsetCommit$")
	child.Env = append(os.Environ(), "INGESTKIT_CRASH_HELPER=1", "CRASH_TOPIC="+topic, "CRASH_GROUP="+group)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = child.Process.Kill() })
	signal := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "DB_COMMITTED") {
				signal <- true
				return
			}
		}
		signal <- false
	}()
	select {
	case ok := <-signal:
		require.True(t, ok)
	case <-ctx.Done():
		t.Fatal("helper did not reach durable commit")
	}
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	before, err := committedOffset(ctx, client, group, topic)
	require.NoError(t, err)
	require.EqualValues(t, -1, before)
	pool := integrationPool(t)
	store := persistence.NewEventStore(pool)
	result := make(chan persistence.WriteResult, 1)
	r, err := NewReader(ReaderConfig{Brokers: []string{brokerAddr(t)}, Topic: topic, DLQTopic: "ingestkit.dlq", GroupID: group}, storeFunc(func(c context.Context, events []event.Envelope) (persistence.WriteResult, error) {
		v, e := store.WriteBatch(c, events)
		result <- v
		return v, e
	}), nil)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	select {
	case v := <-result:
		require.EqualValues(t, 1, v.Duplicates)
		require.Zero(t, v.Inserted)
	case <-ctx.Done():
		t.Fatal("committed event was not replayed")
	}
	for {
		offset, err := committedOffset(ctx, client, group, topic)
		require.NoError(t, err)
		if offset == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("replayed offset not committed")
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
	r.Close()
}
func exerciseQuarantine(t *testing.T, missing bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	client := integrationClient(t)
	topic, dlq, group := "poison-"+uuid.NewString(), "dlq-"+uuid.NewString(), "poison-"+uuid.NewString()
	createTestTopic(t, client, topic)
	first, last := publisherEvent(), publisherEvent()
	a, _ := event.EncodeEnvelope(first)
	b, _ := event.EncodeEnvelope(last)
	require.NoError(t, client.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: a}, &kgo.Record{Topic: topic, Value: []byte("not-json"), Key: []byte("original")}, &kgo.Record{Topic: topic, Value: b}).FirstErr())
	pool := integrationPool(t)
	store := persistence.NewEventStore(pool)
	cfg := ReaderConfig{Brokers: []string{brokerAddr(t)}, Topic: topic, DLQTopic: dlq, GroupID: group}
	if missing {
		r, err := NewReader(cfg, store, nil)
		require.NoError(t, err)
		require.Error(t, r.Run(ctx))
		r.Close()
		offset, err := committedOffset(ctx, client, group, topic)
		require.NoError(t, err)
		require.EqualValues(t, 1, offset)
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", last.EventID.String()).Scan(&n))
		require.Zero(t, n)
	}
	createTestTopic(t, client, dlq)
	r, err := NewReader(cfg, store, nil)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	for {
		offset, err := committedOffset(ctx, client, group, topic)
		require.NoError(t, err)
		if offset == 3 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("failed to resume after restoring DLQ")
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done
	r.Close()
	read, err := kgo.NewClient(kgo.SeedBrokers(brokerAddr(t)), kgo.ConsumeTopics(dlq), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer read.Close()
	probe, end := context.WithTimeout(context.Background(), 5*time.Second)
	defer end()
	records := read.PollRecords(probe, 10).Records()
	require.NotEmpty(t, records)
	var quarantined QuarantinedRecord
	require.NoError(t, json.Unmarshal(records[0].Value, &quarantined))
	require.Equal(t, []byte("not-json"), quarantined.Value)
	require.Equal(t, []byte("original"), quarantined.Key)
	require.EqualValues(t, 1, quarantined.Offset)
}
func TestMalformedRecordQuarantine(t *testing.T)        { exerciseQuarantine(t, false) }
func TestFailedQuarantineCannotSkipSource(t *testing.T) { exerciseQuarantine(t, true) }
func TestConsumerRebalanceRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	client := integrationClient(t)
	topic, group := "rebalance-"+uuid.NewString(), "rebalance-"+uuid.NewString()
	createTestTopic(t, client, topic)
	e := publisherEvent()
	raw, _ := event.EncodeEnvelope(e)
	require.NoError(t, client.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: raw}).FirstErr())
	pool := integrationPool(t)
	store := persistence.NewEventStore(pool)
	blocked := make(chan struct{}, 1)
	firstCtx, stopFirst := context.WithCancel(ctx)
	cfg := ReaderConfig{Brokers: []string{brokerAddr(t)}, Topic: topic, DLQTopic: "ingestkit.dlq", GroupID: group}
	first, err := NewReader(cfg, storeFunc(func(c context.Context, _ []event.Envelope) (persistence.WriteResult, error) {
		blocked <- struct{}{}
		<-c.Done()
		return persistence.WriteResult{}, c.Err()
	}), nil)
	require.NoError(t, err)
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Run(firstCtx) }()
	select {
	case <-blocked:
	case <-ctx.Done():
		t.Fatal("first member not assigned")
	}
	second, err := NewReader(cfg, store, nil)
	require.NoError(t, err)
	secondDone := make(chan error, 1)
	go func() { secondDone <- second.Run(ctx) }()
	stopFirst()
	<-firstDone
	first.Close()
	for {
		offset, err := committedOffset(ctx, client, group, topic)
		if err == nil && offset == 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("new member did not recover unfinished record")
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-secondDone
	second.Close()
}
