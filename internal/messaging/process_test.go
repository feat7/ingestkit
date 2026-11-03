package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/feat7/ingestkit/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"testing"
	"time"
)

type storeFunc func(context.Context, []event.Envelope) (persistence.WriteResult, error)

func (f storeFunc) WriteBatch(c context.Context, e []event.Envelope) (persistence.WriteResult, error) {
	return f(c, e)
}
func sourceRecord(offset int64, valid bool) *kgo.Record {
	raw := []byte("bad")
	if valid {
		raw, _ = event.EncodeEnvelope(publisherEvent())
	}
	return &kgo.Record{Topic: "events", Partition: 0, Offset: offset, Value: raw}
}
func testReader(trace *[]string, failStore, failDLQ bool) *Reader {
	r := &Reader{config: readerDefaults(ReaderConfig{RetryWindow: 10 * time.Millisecond, BackoffMin: time.Millisecond, BackoffMax: time.Millisecond})}
	r.store = storeFunc(func(ctx context.Context, e []event.Envelope) (persistence.WriteResult, error) {
		*trace = append(*trace, "store")
		if failStore {
			return persistence.WriteResult{}, errors.New("db down")
		}
		return persistence.WriteResult{Inserted: int64(len(e))}, nil
	})
	r.commit = func(ctx context.Context, records ...*kgo.Record) error {
		*trace = append(*trace, fmt.Sprintf("commit:%d", records[len(records)-1].Offset))
		return nil
	}
	r.quarantine = func(ctx context.Context, rec *kgo.Record, err error) error {
		*trace = append(*trace, fmt.Sprintf("dlq:%d", rec.Offset))
		if failDLQ {
			return errors.New("dlq down")
		}
		return nil
	}
	r.allow = func() { *trace = append(*trace, "allow") }
	return r
}
func TestProcessCommitsOnlyAfterStorage(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	require.NoError(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true), sourceRecord(2, true)}))
	require.Equal(t, []string{"store", "commit:2"}, trace)
}
func TestProcessDoesNotCommitAfterStorageFailure(t *testing.T) {
	var trace []string
	r := testReader(&trace, true, false)
	require.ErrorIs(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true)}), ErrRetryExhausted)
	for _, s := range trace {
		require.NotContains(t, s, "commit")
	}
}
func TestProcessCannotCommitPastPoisonRecord(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	require.NoError(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true), sourceRecord(1, false), sourceRecord(2, true)}))
	require.Equal(t, []string{"store", "commit:0", "dlq:1", "commit:1", "store", "commit:2"}, trace)
}
func TestQuarantineFailureDoesNotAdvanceSource(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, true)
	require.Error(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true), sourceRecord(1, false), sourceRecord(2, true)}))
	require.Equal(t, []string{"store", "commit:0", "dlq:1"}, trace)
}
func TestRetryCancellation(t *testing.T) {
	var trace []string
	r := testReader(&trace, true, false)
	ctx, c := context.WithCancel(context.Background())
	c()
	require.ErrorIs(t, r.processPartition(ctx, []*kgo.Record{sourceRecord(0, true)}), context.Canceled)
}
func readerFetch(records ...*kgo.Record) kgo.Fetches {
	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "events", Partitions: []kgo.FetchPartition{{Partition: 0, Records: records}}}}}}
}
func TestRetryExhaustionStopsPolling(t *testing.T) {
	var trace []string
	r := testReader(&trace, true, false)
	polls := 0
	r.poll = func(context.Context, int) kgo.Fetches { polls++; return readerFetch(sourceRecord(0, true)) }
	require.Error(t, r.Run(context.Background()))
	require.Equal(t, 1, polls)
}
func TestCommitFailureStopsPolling(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	polls := 0
	r.poll = func(context.Context, int) kgo.Fetches { polls++; return readerFetch(sourceRecord(0, true)) }
	r.commit = func(context.Context, ...*kgo.Record) error { return errors.New("commit failed") }
	require.Error(t, r.Run(context.Background()))
	require.Equal(t, 1, polls)
}
func TestMixedFetchErrorDoesNotSkipValidRecords(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	r.poll = func(context.Context, int) kgo.Fetches {
		f := readerFetch(sourceRecord(0, true))
		f[0].Topics[0].Partitions = append(f[0].Topics[0].Partitions, kgo.FetchPartition{Partition: 1, Err: errors.New("fetch failed")})
		return f
	}
	require.Error(t, r.Run(context.Background()))
	require.Equal(t, []string{"store", "commit:0", "allow"}, trace)
}
func TestRebalanceAlwaysReleased(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, true)
	r.poll = func(context.Context, int) kgo.Fetches { return readerFetch(sourceRecord(0, false)) }
	require.Error(t, r.Run(context.Background()))
	require.Equal(t, "allow", trace[len(trace)-1])
}

func TestUnstoreableEnvelopeQuarantinedBeforeLaterProgress(t *testing.T) {
	for _, payload := range []string{`{"s":"\u0000"}`, `{"s":"\ud800"}`, `{"n":1e1000000}`} {
		var trace []string
		r := testReader(&trace, false, false)
		bad := sourceRecord(1, true)
		e := publisherEvent()
		e.Payload = []byte(payload)
		bad.Value, _ = json.Marshal(e)
		require.NoError(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true), bad, sourceRecord(2, true)}))
		require.Equal(t, []string{"store", "commit:0", "dlq:1", "commit:1", "store", "commit:2"}, trace)
	}
}

func TestProcessingFailureSurvivesReaderRestart(t *testing.T) {
	health := service.NewProcessingHealth()
	observe := func(op Operation) { health.Observe(op.Kind, op.Err) }
	var trace []string
	r := testReader(&trace, true, false)
	r.config.OnOperation = observe
	require.NoError(t, func() error { return nil }(), "dependency Ping succeeds")
	require.Error(t, r.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true)}))
	require.False(t, health.Ready())
	next := testReader(&trace, false, false)
	next.config.OnOperation = observe
	require.False(t, health.Ready(), "new reader must not clear a write failure")
	require.NoError(t, next.processPartition(context.Background(), []*kgo.Record{sourceRecord(0, true)}))
	require.True(t, health.Ready())
	health.Observe("quarantine", errors.New("unavailable"))
	health.Observe("storage", nil)
	require.False(t, health.Ready())
	health.Observe("quarantine", nil)
	require.True(t, health.Ready())
}
func TestStopPollingDrainsCurrentWrite(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	r.stop = make(chan struct{})
	entered, release := make(chan struct{}), make(chan struct{})
	r.store = storeFunc(func(ctx context.Context, es []event.Envelope) (persistence.WriteResult, error) {
		close(entered)
		select {
		case <-ctx.Done():
			return persistence.WriteResult{}, ctx.Err()
		case <-release:
			return persistence.WriteResult{Inserted: int64(len(es))}, nil
		}
	})
	polls := 0
	r.poll = func(context.Context, int) kgo.Fetches { polls++; return readerFetch(sourceRecord(0, true)) }
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background()) }()
	<-entered
	r.StopPolling()
	select {
	case e := <-done:
		t.Fatalf("write was aborted: %v", e)
	default:
	}
	close(release)
	require.NoError(t, <-done)
	require.Equal(t, 1, polls)
	require.Contains(t, trace, "commit:0")
}
func TestStopPollingDeadlineDoesNotClaimCompletion(t *testing.T) {
	var trace []string
	r := testReader(&trace, false, false)
	r.stop = make(chan struct{})
	entered := make(chan struct{})
	r.store = storeFunc(func(ctx context.Context, _ []event.Envelope) (persistence.WriteResult, error) {
		close(entered)
		<-ctx.Done()
		return persistence.WriteResult{}, ctx.Err()
	})
	r.poll = func(context.Context, int) kgo.Fetches { return readerFetch(sourceRecord(0, true)) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	<-entered
	r.StopPolling()
	cancel()
	require.Error(t, <-done)
	require.NotContains(t, trace, "commit:0")
}
