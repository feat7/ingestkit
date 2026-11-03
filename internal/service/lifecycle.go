// Package service provides bounded process shutdown and retry supervision.
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

func Drain(ctx context.Context, stop, flush func(context.Context) error, close func()) error {
	var result error
	for _, fn := range []func(context.Context) error{stop, flush} {
		if err := fn(ctx); err != nil {
			result = errors.Join(result, err)
		}
	}
	done := make(chan struct{})
	go func() { close(); closeChannel(done) }()
	select {
	case <-done:
		return result
	case <-ctx.Done():
		return errors.Join(result, ctx.Err())
	}
}
func closeChannel(c chan struct{}) { close(c) }

// Supervise stops new attempts on stopCtx, while workCtx bounds active work.
func Supervise(stopCtx, workCtx context.Context, attempt func(context.Context) error, backoff time.Duration) error {
	for {
		if workCtx.Err() != nil {
			return workCtx.Err()
		}
		if stopCtx.Err() != nil {
			return nil
		}
		err := attempt(workCtx)
		if workCtx.Err() != nil {
			return errors.Join(err, workCtx.Err())
		}
		if stopCtx.Err() != nil {
			return err
		}
		timer := time.NewTimer(backoff)
		select {
		case <-stopCtx.Done():
			timer.Stop()
			return err
		case <-workCtx.Done():
			timer.Stop()
			return workCtx.Err()
		case <-timer.C:
		}
	}
}
func Check(ctx context.Context, address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+net.JoinHostPort(host, port)+"/ready", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("service not ready: HTTP %d", res.StatusCode)
	}
	return nil
}
