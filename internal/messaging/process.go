package messaging

import (
	"context"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/twmb/franz-go/pkg/kgo"
	"time"
)

func (r *Reader) processPartition(ctx context.Context, records []*kgo.Record) error {
	var valid []event.Envelope
	var source []*kgo.Record
	flush := func() error {
		if len(source) == 0 {
			return nil
		}
		if err := r.persist(ctx, valid, source[0]); err != nil {
			return err
		}
		commitCtx, cancel := context.WithTimeout(ctx, r.config.AttemptTimeout)
		defer cancel()
		err := r.commit(commitCtx, source...)
		r.notify("progress", source[len(source)-1], err)
		if err != nil {
			return fmt.Errorf("commit source progress: %w", err)
		}
		valid = nil
		source = nil
		return nil
	}
	for _, rec := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		e, err := event.DecodeEnvelope(rec.Value)
		if err != nil {
			if err := flush(); err != nil {
				return err
			}
			quarantineErr := r.quarantine(ctx, rec, err)
			r.notify("quarantine", rec, quarantineErr)
			if quarantineErr != nil {
				return quarantineErr
			}
			commitCtx, cancel := context.WithTimeout(ctx, r.config.AttemptTimeout)
			err = r.commit(commitCtx, rec)
			r.notify("progress", rec, err)
			cancel()
			if err != nil {
				return err
			}
			continue
		}
		valid = append(valid, e)
		source = append(source, rec)
		if len(valid) >= r.config.BatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
func (r *Reader) persist(ctx context.Context, events []event.Envelope, source *kgo.Record) error {
	window, cancel := context.WithTimeout(ctx, r.config.RetryWindow)
	defer cancel()
	backoff := r.config.BackoffMin
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		attempt, end := context.WithTimeout(window, r.config.AttemptTimeout)
		start := time.Now()
		res, err := r.store.WriteBatch(attempt, events)
		r.notify("storage", source, err)
		end()
		if err == nil {
			if r.metrics != nil {
				received := make([]time.Time, len(events))
				for i, e := range events {
					received[i] = e.ReceivedAt
				}
				r.metrics.ObserveWrite(res, received, time.Since(start))
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if r.metrics != nil {
			r.metrics.ObserveRetry()
		}
		timer := time.NewTimer(backoff)
		select {
		case <-window.Done():
			timer.Stop()
			return fmt.Errorf("%w: %v", ErrRetryExhausted, err)
		case <-timer.C:
		}
		if backoff < r.config.BackoffMax {
			backoff *= 2
			if backoff > r.config.BackoffMax {
				backoff = r.config.BackoffMax
			}
		}
	}
}
