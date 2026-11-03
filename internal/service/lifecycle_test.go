package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAPIShutdownStopsAdmissionBeforeFlush(t *testing.T) {
	var order []string
	ctx, c := context.WithTimeout(context.Background(), time.Second)
	defer c()
	require.NoError(t, Drain(ctx, func(context.Context) error { order = append(order, "stop"); return nil }, func(context.Context) error { order = append(order, "flush"); return nil }, func() { order = append(order, "close") }))
	require.Equal(t, []string{"stop", "flush", "close"}, order)
}
func TestShutdownTimeoutIsFailure(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer c()
	require.Error(t, Drain(ctx, func(c context.Context) error { <-c.Done(); return c.Err() }, func(context.Context) error { return nil }, func() {}))
}
func TestConsumerSupervisorRejoinsAfterFailure(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	attempts := 0
	err := Supervise(ctx, func(context.Context) error {
		attempts++
		if attempts == 2 {
			c()
		}
		return errors.New("temporary")
	}, time.Millisecond)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, attempts)
}
