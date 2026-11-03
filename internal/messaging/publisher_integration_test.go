//go:build integration

package messaging

import (
	"context"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"os"
	"testing"
	"time"
)

func brokerAddr(t *testing.T) string {
	t.Helper()
	s := os.Getenv("INGESTKIT_TEST_BROKER")
	require.NotEmpty(t, s, "real broker required")
	return s
}
func TestPublisherRoundTrip(t *testing.T) {
	addr := brokerAddr(t)
	p, err := NewPublisher(PublisherConfig{Brokers: []string{addr}, Topic: "ingestkit.events"}, nil)
	require.NoError(t, err)
	e := publisherEvent()
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	require.NoError(t, p.Ping(ctx))
	require.NoError(t, p.Publish(ctx, []event.Envelope{e}))
	p.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(addr), kgo.ConsumeTopics("ingestkit.events"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer client.Close()
	found := false
	for !found && ctx.Err() == nil {
		fetch := client.PollRecords(ctx, 500)
		fetch.EachRecord(func(r *kgo.Record) {
			got, er := event.DecodeEnvelope(r.Value)
			if er == nil && got.EventID == e.EventID {
				found = true
			}
		})
	}
	require.True(t, found)
}
func TestPublisherBrokerUnavailable(t *testing.T) {
	p, err := NewPublisher(PublisherConfig{Brokers: []string{"127.0.0.1:1"}, Topic: "ingestkit.events", PublishTimeout: 100 * time.Millisecond}, nil)
	require.NoError(t, err)
	defer p.Close()
	start := time.Now()
	require.Error(t, p.Publish(context.Background(), []event.Envelope{publisherEvent()}))
	require.Less(t, time.Since(start), time.Second)
}
func TestPublisherNoAutoTopicCreation(t *testing.T) {
	p, err := NewPublisher(PublisherConfig{Brokers: []string{brokerAddr(t)}, Topic: "missing-" + uuid.NewString()}, nil)
	require.NoError(t, err)
	defer p.Close()
	ctx, c := context.WithTimeout(context.Background(), time.Second)
	defer c()
	require.Error(t, p.Ping(ctx))
}
func TestPublisherBufferLimitIntegration(t *testing.T) {
	p, err := NewPublisher(PublisherConfig{Brokers: []string{"127.0.0.1:1"}, Topic: "ingestkit.events", MaxBufferedRecords: 1, PublishTimeout: 100 * time.Millisecond}, nil)
	require.NoError(t, err)
	defer p.Close()
	require.ErrorIs(t, p.Publish(context.Background(), []event.Envelope{publisherEvent(), publisherEvent()}), ErrSaturated)
}
