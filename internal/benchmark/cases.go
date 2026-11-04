package benchmark

import (
	"fmt"
	"time"
)

type Case struct {
	HTTPMaxConnsPerHost, HTTPMaxIdleConnsPerHost                   int
	ID, Kind, Recovery, Pattern, PayloadProfile                    string
	Count, BatchSize, PayloadBytes, Rate, Concurrency, Repetitions int
	DuplicateFraction                                              float64
	Window, Warmup, Drain, Poll, FaultAt, RestoreAt                time.Duration
}

func StudyCases() []Case {
	var cases []Case
	db := func(id string, batch, payload int, duplicates float64) {
		cases = append(cases, Case{ID: id, Kind: "database", Count: 5000, BatchSize: batch, PayloadBytes: payload, DuplicateFraction: duplicates, Repetitions: 3, PayloadProfile: "varied"})
	}
	for _, batch := range []int{1, 10, 50, 100, 500, 1000} {
		db(fmt.Sprintf("db-batch-%d", batch), batch, 256, 0)
	}
	for _, payload := range []int{1024, 16384} {
		db(fmt.Sprintf("db-payload-%d", payload), 100, payload, 0)
	}
	for _, duplicate := range []int{10, 50, 90} {
		db(fmt.Sprintf("db-duplicates-%d", duplicate), 100, 256, float64(duplicate)/100)
	}
	base := Case{Kind: "pipeline", Pattern: "steady", PayloadProfile: "varied", Count: 10000, BatchSize: 100, PayloadBytes: 256, Rate: 1000, Concurrency: 32, Repetitions: 3, Window: 10 * time.Second, Warmup: 2 * time.Second, Drain: 30 * time.Second, Poll: 50 * time.Millisecond}
	for _, batch := range []int{1, 10, 100} {
		c := base
		c.ID = fmt.Sprintf("pipeline-batch-%d", batch)
		c.BatchSize = batch
		cases = append(cases, c)
	}
	for _, rate := range []int{100, 5000, 10000} {
		c := base
		c.ID = fmt.Sprintf("pipeline-rate-%d", rate)
		c.Rate = rate
		c.Count = rate * 10
		cases = append(cases, c)
	}
	for _, payload := range []int{1024, 16384} {
		c := base
		c.ID = fmt.Sprintf("pipeline-payload-%d", payload)
		c.PayloadBytes = payload
		c.BatchSize = 10
		cases = append(cases, c)
	}
	c := base
	c.ID = "pipeline-burst"
	c.Pattern = "burst"
	cases = append(cases, c)
	for _, concurrency := range []int{8, 64} {
		c := base
		c.ID = fmt.Sprintf("pipeline-concurrency-%d", concurrency)
		c.Concurrency = concurrency
		c.BatchSize = 1
		cases = append(cases, c)
	}
	for _, recovery := range []string{"database", "consumer"} {
		c := base
		c.ID = "recovery-" + recovery
		c.Kind = "recovery"
		c.Recovery = recovery
		c.Rate = 500
		c.Window = 15 * time.Second
		c.Count = 7500
		c.FaultAt = 3 * time.Second
		c.RestoreAt = 8 * time.Second
		if recovery == "consumer" {
			c.RestoreAt = 5 * time.Second
		}
		cases = append(cases, c)
	}
	cases = append(cases, Case{ID: "recovery-outbox", Kind: "recovery", Recovery: "outbox", PayloadProfile: "varied", Count: 100, BatchSize: 100, PayloadBytes: 256, Repetitions: 3, RestoreAt: 8 * time.Second, Drain: 30 * time.Second, Poll: 50 * time.Millisecond})
	return cases
}
func smokeCases() []Case {
	return []Case{
		{ID: "smoke-database", Kind: "database", PayloadProfile: "varied", Count: 100, BatchSize: 10, PayloadBytes: 256, Repetitions: 3},
		{ID: "smoke-pipeline", Kind: "pipeline", Pattern: "steady", PayloadProfile: "varied", Count: 10, Rate: 20, Concurrency: 2, BatchSize: 2, PayloadBytes: 256, Repetitions: 3, Window: 500 * time.Millisecond, Warmup: 100 * time.Millisecond, Drain: 5 * time.Second, Poll: 25 * time.Millisecond},
	}
}
