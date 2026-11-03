package messaging

import (
	"context"
	"encoding/json"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/twmb/franz-go/pkg/kgo"
	"time"
)

type QuarantinedRecord struct {
	Topic     string             `json:"topic"`
	Partition int32              `json:"partition"`
	Offset    int64              `json:"offset"`
	Key       []byte             `json:"key"`
	Headers   []kgo.RecordHeader `json:"headers"`
	Value     []byte             `json:"value"`
	Error     string             `json:"error"`
	FailedAt  time.Time          `json:"failed_at"`
}
type Quarantine struct {
	publisher *Publisher
	metrics   *telemetry.Metrics
}

func (q *Quarantine) Write(ctx context.Context, r *kgo.Record, cause error) (err error) {
	defer func() {
		if q.metrics != nil {
			outcome := "success"
			if err != nil {
				outcome = "error"
			}
			q.metrics.ObserveQuarantine(outcome)
		}
	}()
	message := cause.Error()
	if len(message) > 1024 {
		message = message[:1024]
	}
	raw, err := json.Marshal(QuarantinedRecord{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset, Key: r.Key, Headers: r.Headers, Value: r.Value, Error: message, FailedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	return q.publisher.send(ctx, []*kgo.Record{{Topic: q.publisher.topic, Key: append([]byte(nil), r.Key...), Value: raw}})
}
