package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"strings"
	"testing"
)

func TestQuarantinePreservesSourceAndBoundsError(t *testing.T) {
	var raw []byte
	p := testPublisher(func(ctx context.Context, r *kgo.Record, cb func(*kgo.Record, error)) { raw = r.Value; cb(r, nil) })
	q := &Quarantine{publisher: p}
	rec := sourceRecord(7, false)
	rec.Key = []byte{0, 255}
	rec.Headers = []kgo.RecordHeader{{Key: "trace", Value: []byte("a")}}
	require.NoError(t, q.Write(context.Background(), rec, errors.New(strings.Repeat("x", 2000))))
	var got QuarantinedRecord
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, rec.Value, got.Value)
	require.Equal(t, rec.Key, got.Key)
	require.Equal(t, int64(7), got.Offset)
	require.Len(t, got.Error, 1024)
}
