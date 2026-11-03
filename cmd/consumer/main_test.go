package main

import (
	"context"
	"errors"
	"github.com/feat7/ingestkit/internal/service"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessFailsWhenPingWorksButWritingFails(t *testing.T) {
	h := service.NewProcessingHealth()
	ping := func(context.Context) error { return nil }
	handler := readiness(h, ping, ping)
	h.Observe("storage", errors.New("write denied"))
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/ready", nil))
	require.Equal(t, 503, w.Code)
	h.Observe("storage", nil)
	w = httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/ready", nil))
	require.Equal(t, 200, w.Code)
}
func TestConsumerDrainFinishesWriteOrReportsDeadline(t *testing.T) {
	for _, finish := range []bool{true, false} {
		t.Run(map[bool]string{true: "finish", false: "deadline"}[finish], func(t *testing.T) {
			work, cancelWork := context.WithCancel(context.Background())
			defer cancelWork()
			stopped, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			go func() {
				select {
				case <-release:
					done <- nil
				case <-work.Done():
					done <- work.Err()
				}
			}()
			budget, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- drainConsumer(budget, done, func() { close(stopped) }, cancelWork) }()
			<-stopped
			require.NoError(t, work.Err(), "poll stop must not cancel active write")
			if finish {
				close(release)
				require.NoError(t, <-result)
			} else {
				require.ErrorIs(t, <-result, context.DeadlineExceeded)
				require.Error(t, work.Err())
			}
		})
	}
	failure := errors.New("incomplete write")
	done := make(chan error, 1)
	done <- failure
	require.ErrorIs(t, drainConsumer(context.Background(), done, func() {}, func() {}), failure)
}
