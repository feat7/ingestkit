package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
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
