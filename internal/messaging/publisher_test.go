package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"testing"
	"time"
)

func publisherEvent() event.Envelope {
	return event.Envelope{Input: event.Input{EventID: uuid.New(), EventType: "test", OccurredAt: time.Now().UTC(), Payload: json.RawMessage("{\"x\":1}")}, TenantID: "test", ReceivedAt: time.Now().UTC()}
}
func testPublisher(fn func(context.Context, *kgo.Record, func(*kgo.Record, error))) *Publisher {
	return &Publisher{topic: "events", timeout: time.Second, produce: fn}
}
func TestPublishWaitsForEveryAck(t *testing.T) {
	callbacks := make(chan func(), 2)
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) {
		callbacks <- func() { cb(r, nil) }
	})
	done := make(chan error, 1)
	go func() { done <- p.Publish(context.Background(), []event.Envelope{publisherEvent(), publisherEvent()}) }()
	a, b := <-callbacks, <-callbacks
	a()
	select {
	case <-done:
		t.Fatal("returned before second ack")
	default:
	}
	b()
	require.NoError(t, <-done)
}
func TestPublishFailsForAnyRecordError(t *testing.T) {
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) {
		cb(r, errors.New("broker failed"))
	})
	require.ErrorIs(t, p.Publish(context.Background(), []event.Envelope{publisherEvent()}), ErrUnavailable)
}
func TestPublishDeadlineDoesNotWaitForLateCallback(t *testing.T) {
	p := testPublisher(func(context.Context, *kgo.Record, func(*kgo.Record, error)) {})
	ctx, c := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer c()
	start := time.Now()
	require.ErrorIs(t, p.Publish(ctx, []event.Envelope{publisherEvent()}), ErrUnavailable)
	require.Less(t, time.Since(start), time.Second)
}
func TestLateCallbackIsNonblocking(t *testing.T) {
	callback := make(chan func(), 1)
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) {
		callback <- func() { cb(r, nil) }
	})
	ctx, c := context.WithCancel(context.Background())
	c()
	require.Error(t, p.Publish(ctx, []event.Envelope{publisherEvent()}))
	done := make(chan struct{})
	go func() { (<-callback)(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("late callback blocked")
	}
}
func TestPublishReportsBufferSaturation(t *testing.T) {
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) { cb(r, kgo.ErrMaxBuffered) })
	require.ErrorIs(t, p.Publish(context.Background(), []event.Envelope{publisherEvent()}), ErrSaturated)
}
func TestPublishOwnsRecordBytes(t *testing.T) {
	var record *kgo.Record
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) { record = r; cb(r, nil) })
	e := publisherEvent()
	require.NoError(t, p.Publish(context.Background(), []event.Envelope{e}))
	e.Payload[0] = '!'
	require.Contains(t, string(record.Value), "{\"x\":1}")
}
func TestPublisherRejectsMissingTopic(t *testing.T) {
	_, err := NewPublisher(PublisherConfig{Brokers: []string{"localhost:9092"}}, nil)
	require.Error(t, err)
}
