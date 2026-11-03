package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PipelineVerification struct {
	StoredIDs                            []uuid.UUID
	Missing, Unexpected, ContentMismatch int
}

// VerifyPipelineRows is independent of pending-observer bookkeeping. It includes
// unplanned IDs in the case's event type and expected IDs even if their type changed.
func VerifyPipelineRows(ctx context.Context, p *pgxpool.Pool, tenant string, expected []event.Envelope, required []uuid.UUID) (PipelineVerification, error) {
	var v PipelineVerification
	if len(expected) == 0 {
		return v, errors.New("no expected pipeline dataset")
	}
	ids := make([]uuid.UUID, len(expected))
	for i, e := range expected {
		ids[i] = e.EventID
	}
	return verifySelectedRows(ctx, p, expected, required, "SELECT event_id,event_type,occurred_at,payload FROM ingestkit.events WHERE tenant_id=$1 AND (event_type=$2 OR event_id=ANY($3::uuid[]))", tenant, expected[0].EventType, ids)
}
func VerifyOutboxRows(ctx context.Context, p *pgxpool.Pool, tenant, prefix string, expected []event.Envelope, required []uuid.UUID) (PipelineVerification, error) {
	if len(expected) == 0 || prefix == "" {
		return PipelineVerification{}, errors.New("missing outbox namespace")
	}
	ids := make([]uuid.UUID, len(expected))
	for i, e := range expected {
		ids[i] = e.EventID
	}
	return verifySelectedRows(ctx, p, expected, required, "SELECT event_id,event_type,occurred_at,payload FROM ingestkit.events WHERE tenant_id=$1 AND (event_id=ANY($2::uuid[]) OR left(payload->>'order_id',char_length($3))=$3)", tenant, ids, prefix)
}
func verifySelectedRows(ctx context.Context, p *pgxpool.Pool, expected []event.Envelope, required []uuid.UUID, query string, args ...any) (PipelineVerification, error) {
	var v PipelineVerification
	want := expectedEvents(expected)
	rows, err := p.Query(ctx, query, args...)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	seen := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		var typ string
		var occurred time.Time
		var payload []byte
		if err = rows.Scan(&id, &typ, &occurred, &payload); err != nil {
			return v, err
		}
		v.StoredIDs = append(v.StoredIDs, id)
		seen[id] = true
		e, ok := want[id]
		if !ok {
			v.Unexpected++
			continue
		}
		var a, b any
		if json.Unmarshal(payload, &a) != nil || json.Unmarshal(e.Payload, &b) != nil || !reflect.DeepEqual(a, b) || typ != e.EventType || !occurred.Equal(e.OccurredAt) {
			v.ContentMismatch++
		}
	}
	if err = rows.Err(); err != nil {
		return v, err
	}
	for _, id := range required {
		if !seen[id] {
			v.Missing++
		}
	}
	if v.Missing+v.Unexpected+v.ContentMismatch > 0 {
		return v, errors.New("independent pipeline ID/content verification failed")
	}
	return v, nil
}
