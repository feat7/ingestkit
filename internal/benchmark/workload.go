package benchmark

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"math"
	"strings"
	"time"
)

type WorkloadConfig struct {
	Namespace         string  `json:"namespace,omitempty"`
	PayloadProfile    string  `json:"payload_profile,omitempty"`
	Seed              int64   `json:"seed"`
	Count             int     `json:"count"`
	PayloadBytes      int     `json:"payload_bytes"`
	DuplicateFraction float64 `json:"duplicate_fraction"`
}

// Workload contains exactly Count submissions; duplicates repeat the original bytes.
// All generation and JSON encoding happens before the database timer starts.
func Workload(c WorkloadConfig) ([]event.Envelope, error) {
	if c.Count <= 0 || c.Count > 1000000 || len(c.Namespace) > 256 || (c.PayloadProfile != "" && c.PayloadProfile != "fixed" && c.PayloadProfile != "varied") || math.IsNaN(c.DuplicateFraction) || math.IsInf(c.DuplicateFraction, 0) || c.PayloadBytes < 16 || c.PayloadBytes > event.MaxEventBytes-2048 || c.DuplicateFraction < 0 || c.DuplicateFraction >= 1 {
		return nil, errors.New("invalid workload count, payload size or duplicate fraction")
	}
	unique := c.Count - int(float64(c.Count)*c.DuplicateFraction)
	out := make([]event.Envelope, c.Count)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < unique; i++ {
		prefix := fmt.Sprintf("%x", uint64(c.Seed)^uint64(i))
		padding := strings.Repeat("x", c.PayloadBytes-11)
		if c.PayloadProfile == "varied" {
			alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
			data := make([]byte, c.PayloadBytes-11)
			for offset := 0; offset < len(data); {
				sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d/%d", c.Namespace, c.Seed, i, offset)))
				for _, b := range sum {
					if offset == len(data) {
						break
					}
					data[offset] = alphabet[int(b)&63]
					offset++
				}
			}
			padding = string(data)
		}
		payload, _ := json.Marshal(map[string]string{"data": padding})
		// Include the seed in bounded data while preserving the requested encoded length.
		if c.PayloadProfile != "varied" && len(prefix) <= len(padding) {
			payload, _ = json.Marshal(map[string]string{"data": prefix + padding[len(prefix):]})
		}
		identity := fmt.Sprintf("%d/%d", c.Seed, i)
		typ := "checkout_event"
		if c.Namespace != "" {
			identity = c.Namespace + "/" + identity
			sum := sha256.Sum256([]byte(c.Namespace))
			typ = fmt.Sprintf("bench_%x", sum[:8])
		}
		out[i] = event.Envelope{Input: event.Input{EventID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)), EventType: typ, OccurredAt: at.Add(time.Duration(i) * time.Microsecond), Payload: payload}, TenantID: "benchmark", ReceivedAt: at}
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
