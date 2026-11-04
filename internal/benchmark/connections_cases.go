package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"time"
)

func ConnectionCases() []Case {
	base := Case{Kind: "pipeline", Pattern: "steady", PayloadProfile: "varied", Count: 150000, BatchSize: 100, PayloadBytes: 1024, Rate: 15000, Concurrency: 128, HTTPMaxConnsPerHost: 128, HTTPMaxIdleConnsPerHost: 128, Repetitions: 3, Window: 10 * time.Second, Warmup: 2 * time.Second, Drain: 30 * time.Second, Poll: time.Second}
	result := []Case{}
	for _, cap := range []int{4, 8, 16, 32, 64, 128, 0} {
		c := base
		c.ID = fmt.Sprintf("connections-cap-%d", cap)
		c.HTTPMaxConnsPerHost = cap
		result = append(result, c)
	}
	for _, rate := range []int{30000, 50000} {
		c := base
		c.ID = fmt.Sprintf("connections-rate-%d", rate)
		c.Rate = rate
		c.Count = rate * 10
		result = append(result, c)
	}
	for _, payload := range []int{256, 1024, 16384} {
		c := base
		c.ID = fmt.Sprintf("connections-payload-%d", payload)
		c.PayloadBytes = payload
		c.BatchSize = 10
		c.Rate = 1000
		c.Count = 10000
		result = append(result, c)
	}
	return result
}

// Validate before launching infrastructure; retain the existing one-million
// event guard and the same public API body limit for every payload size.
func ValidatePipelineWorkload(c PipelineConfig) error {
	count := float64(c.Rate) * c.Duration.Seconds()
	if c.Rate <= 0 || c.Duration <= 0 || count < 1 || count > 1000000 {
		return errors.New("pipeline window requires 1..1000000 events; sustained 60s/50k needs a separately bounded runner")
	}
	if c.BatchSize < 1 || c.BatchSize > 100 {
		return errors.New("HTTP batch must contain 1..100 events")
	}
	es, err := Workload(WorkloadConfig{Count: c.BatchSize, PayloadBytes: c.PayloadBytes, Namespace: c.Namespace})
	if err != nil {
		return err
	}
	inputs := make([]event.Input, len(es))
	for i, e := range es {
		inputs[i] = e.Input
	}
	raw, err := json.Marshal(map[string]any{"events": inputs})
	if err != nil {
		return err
	}
	if len(raw) > event.MaxRequestBytes {
		return errors.New("encoded request exceeds API body limit; reduce HTTP batch size")
	}
	return nil
}
