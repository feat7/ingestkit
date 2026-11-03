package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/twmb/franz-go/pkg/kgo"
	"time"
)

var ErrRetryExhausted = errors.New("storage retry window exhausted")

type BatchStore interface {
	WriteBatch(context.Context, []event.Envelope) (persistence.WriteResult, error)
}
type ReaderConfig struct {
	Brokers                                             []string
	Topic, DLQTopic, GroupID                            string
	BatchSize                                           int
	AttemptTimeout, RetryWindow, BackoffMin, BackoffMax time.Duration
}
type Reader struct {
	client     *kgo.Client
	publisher  *Publisher
	config     ReaderConfig
	store      BatchStore
	metrics    *telemetry.Metrics
	poll       func(context.Context, int) kgo.Fetches
	commit     func(context.Context, ...*kgo.Record) error
	allow      func()
	quarantine func(context.Context, *kgo.Record, error) error
}

func readerDefaults(c ReaderConfig) ReaderConfig {
	if c.BatchSize == 0 {
		c.BatchSize = 500
	}
	if c.AttemptTimeout == 0 {
		c.AttemptTimeout = 5 * time.Second
	}
	if c.RetryWindow == 0 {
		c.RetryWindow = 15 * time.Second
	}
	if c.BackoffMin == 0 {
		c.BackoffMin = 100 * time.Millisecond
	}
	if c.BackoffMax == 0 {
		c.BackoffMax = 2 * time.Second
	}
	return c
}
func NewReader(c ReaderConfig, s BatchStore, m *telemetry.Metrics) (*Reader, error) {
	c = readerDefaults(c)
	if len(c.Brokers) == 0 || c.Topic == "" || c.DLQTopic == "" || c.GroupID == "" || s == nil || c.BatchSize < 1 || c.BatchSize > 500 || c.AttemptTimeout <= 0 || c.RetryWindow <= 0 || c.RetryWindow > 15*time.Second || c.BackoffMin <= 0 || c.BackoffMax < c.BackoffMin {
		return nil, errors.New("invalid reader configuration")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(c.Brokers...), kgo.ConsumerGroup(c.GroupID), kgo.ConsumeTopics(c.Topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), kgo.DisableAutoCommit(), kgo.BlockRebalanceOnPoll(), kgo.RebalanceTimeout(60*time.Second), kgo.FetchMaxBytes(4<<20), kgo.FetchMaxPartitionBytes(4<<20), kgo.MaxConcurrentFetches(1))
	if err != nil {
		return nil, err
	}
	pub, err := NewPublisher(PublisherConfig{Brokers: c.Brokers, Topic: c.DLQTopic}, m)
	if err != nil {
		client.CloseAllowingRebalance()
		return nil, err
	}
	q := &Quarantine{publisher: pub, metrics: m}
	return &Reader{client: client, publisher: pub, config: c, store: s, metrics: m, poll: client.PollRecords, commit: client.CommitRecords, allow: client.AllowRebalance, quarantine: q.Write}, nil
}
func (r *Reader) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		fetches := r.poll(ctx, r.config.BatchSize)
		if err := r.processFetch(ctx, fetches); err != nil {
			return err
		}
	}
	return ctx.Err()
}
func (r *Reader) processFetch(ctx context.Context, f kgo.Fetches) error {
	defer r.allow()
	cycle, cancel := context.WithTimeout(ctx, r.config.RetryWindow)
	defer cancel()
	var result error
	f.EachPartition(func(p kgo.FetchTopicPartition) {
		if result != nil {
			return
		}
		if len(p.Records) > 0 {
			result = r.processPartition(cycle, p.Records)
		}
	})
	if result != nil {
		return result
	}
	if errs := f.Errors(); len(errs) > 0 {
		return fmt.Errorf("fetch failure: %w", errs[0].Err)
	}
	return ctx.Err()
}
func (r *Reader) Ping(ctx context.Context) error {
	if err := topicReady(ctx, r.client, r.config.Topic); err != nil {
		return err
	}
	return r.publisher.Ping(ctx)
}
func (r *Reader) Close() { r.client.CloseAllowingRebalance(); r.publisher.Close() }
