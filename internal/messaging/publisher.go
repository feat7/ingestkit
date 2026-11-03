package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"time"
)

var (
	ErrSaturated   = errors.New("broker buffer saturated")
	ErrUnavailable = errors.New("broker publication unavailable")
)

type PublisherConfig struct {
	Brokers                              []string
	Topic                                string
	MaxBufferedBytes, MaxBufferedRecords int
	PublishTimeout                       time.Duration
}
type Publisher struct {
	client  *kgo.Client
	topic   string
	timeout time.Duration
	metrics *telemetry.Metrics
	produce func(context.Context, *kgo.Record, func(*kgo.Record, error))
}

func NewPublisher(c PublisherConfig, m *telemetry.Metrics) (*Publisher, error) {
	if len(c.Brokers) == 0 || c.Topic == "" {
		return nil, errors.New("brokers and topic are required")
	}
	if c.MaxBufferedBytes == 0 {
		c.MaxBufferedBytes = 64 << 20
	}
	if c.MaxBufferedRecords == 0 {
		c.MaxBufferedRecords = 1000
	}
	if c.PublishTimeout == 0 {
		c.PublishTimeout = 5 * time.Second
	}
	if c.MaxBufferedBytes < 1 || c.MaxBufferedRecords < 1 || c.PublishTimeout < time.Millisecond {
		return nil, errors.New("invalid publisher limits")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(c.Brokers...), kgo.RequiredAcks(kgo.AllISRAcks()), kgo.MaxBufferedBytes(c.MaxBufferedBytes), kgo.MaxBufferedRecords(c.MaxBufferedRecords), kgo.RecordDeliveryTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	return &Publisher{client: client, topic: c.Topic, timeout: c.PublishTimeout, metrics: m, produce: client.TryProduce}, nil
}
func (p *Publisher) Publish(ctx context.Context, events []event.Envelope) error {
	records := make([]*kgo.Record, len(events))
	for i, e := range events {
		raw, err := event.EncodeEnvelope(e)
		if err != nil {
			return err
		}
		records[i] = &kgo.Record{Topic: p.topic, Key: []byte(e.TenantID + ":" + e.EventID.String()), Value: raw}
	}
	return p.send(ctx, records)
}
func (p *Publisher) send(ctx context.Context, records []*kgo.Record) (err error) {
	start := time.Now()
	defer func() {
		if p.metrics != nil {
			o := "success"
			if err != nil {
				o = "error"
			}
			p.metrics.ObservePublish(o, time.Since(start))
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	results := make(chan error, len(records))
	for _, r := range records {
		p.produce(ctx, r, func(_ *kgo.Record, e error) { results <- e })
	}
	for range records {
		select {
		case e := <-results:
			if e != nil {
				if errors.Is(e, kgo.ErrMaxBuffered) {
					return fmt.Errorf("%w: retry with same event IDs", ErrSaturated)
				}
				return fmt.Errorf("%w: %v", ErrUnavailable, e)
			}
		case <-ctx.Done():
			return fmt.Errorf("%w: %v; delivery may have occurred, retry with same IDs", ErrUnavailable, ctx.Err())
		}
	}
	return nil
}
func topicReady(ctx context.Context, c *kgo.Client, topic string) error {
	req := kmsg.NewPtrMetadataRequest()
	req.AllowAutoTopicCreation = false
	req.Topics = []kmsg.MetadataRequestTopic{{Topic: &topic}}
	response, err := c.Request(ctx, req)
	if err != nil {
		return err
	}
	r := response.(*kmsg.MetadataResponse)
	if len(r.Topics) != 1 || r.Topics[0].ErrorCode != 0 || len(r.Topics[0].Partitions) == 0 {
		return errors.New("broker topic is not ready")
	}
	for _, part := range r.Topics[0].Partitions {
		if part.ErrorCode != 0 || part.Leader < 0 {
			return errors.New("topic partition is not ready")
		}
	}
	return nil
}
func (p *Publisher) Ping(ctx context.Context) error  { return topicReady(ctx, p.client, p.topic) }
func (p *Publisher) Flush(ctx context.Context) error { return p.client.Flush(ctx) }
func (p *Publisher) Close()                          { p.client.Close() }
