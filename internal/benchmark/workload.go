package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"strings"
	"time"
)

type WorkloadConfig struct {
	Seed              int64   `json:"seed"`
	Count             int     `json:"count"`
	PayloadBytes      int     `json:"payload_bytes"`
	DuplicateFraction float64 `json:"duplicate_fraction"`
}

// Workload contains exactly Count submissions; duplicates repeat the original bytes.
// All generation and JSON encoding happens before the database timer starts.
func Workload(c WorkloadConfig) ([]event.Envelope, error) {
	if c.Count <= 0 || c.PayloadBytes < 16 || c.PayloadBytes > event.MaxEventBytes-2048 || c.DuplicateFraction < 0 || c.DuplicateFraction >= 1 {
		return nil, errors.New("invalid workload count, payload size or duplicate fraction")
	}
	unique := c.Count - int(float64(c.Count)*c.DuplicateFraction)
	out := make([]event.Envelope, c.Count)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < unique; i++ {
		prefix := fmt.Sprintf("%x", uint64(c.Seed)^uint64(i))
		padding := strings.Repeat("x", c.PayloadBytes-11)
		payload, _ := json.Marshal(map[string]string{"data": padding})
		// Include the seed in bounded data while preserving the requested encoded length.
		if len(prefix) <= len(padding) {
			payload, _ = json.Marshal(map[string]string{"data": prefix + padding[len(prefix):]})
		}
		out[i] = event.Envelope{Input: event.Input{EventID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%d/%d", c.Seed, i))), EventType: "checkout_event", OccurredAt: at.Add(time.Duration(i) * time.Microsecond), Payload: payload}, TenantID: "benchmark", ReceivedAt: at}
	}
	for i := unique; i < c.Count; i++ {
		out[i] = out[(i-unique)%unique]
	}
	return out, nil
}
func expectedEvents(es []event.Envelope) map[uuid.UUID]event.Envelope {
	m := map[uuid.UUID]event.Envelope{}
	for _, e := range es {
		if _, ok := m[e.EventID]; !ok {
			m[e.EventID] = e
		}
	}
	return m
}
