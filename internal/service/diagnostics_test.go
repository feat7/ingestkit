package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsRedactsAndBoundsRepeatedFailures(t *testing.T) {
	var out bytes.Buffer
	d := NewDiagnostics(slog.New(slog.NewJSONHandler(&out, nil)), 10*time.Second)
	now := time.Unix(100, 0)
	d.now = func() time.Time { return now }
	err := &pgconn.PgError{Code: "42501", Message: "secret-api-key payload-canary postgres://private:password@host/db", Detail: "payload-canary"}
	for i := 0; i < 100; i++ {
		d.Report("storage", "ingestkit.events", 2, 80, err)
	}
	require.Equal(t, 1, strings.Count(out.String(), "\n"))
	require.Contains(t, out.String(), `"sqlstate":"42501"`)
	now = now.Add(11 * time.Second)
	d.Report("storage", "ingestkit.events", 2, 80, err)
	require.Equal(t, 2, strings.Count(out.String(), "\n"))
	require.Contains(t, out.String(), `"suppressed":99`)
	d.Report("storage", "ingestkit.events", 2, 80, nil)
	d.Report("storage", "ingestkit.events", 2, 80, nil)
	require.Equal(t, 3, strings.Count(out.String(), "\n"))
	require.Contains(t, out.String(), "recovered")
	d.Report("storage", "ingestkit.events", 2, 81, context.DeadlineExceeded)
	require.Equal(t, 4, strings.Count(out.String(), "\n"))
	require.Contains(t, out.String(), `"error_class":"deadline"`)
	d.Report("unknown-kind-secret", "payload-canary", -1, -1, errors.New("payload-canary"))
	for _, secret := range []string{"secret-api-key", "payload-canary", "postgres://", "private:password", "unknown-kind-secret"} {
		require.NotContains(t, out.String(), secret)
	}
}
