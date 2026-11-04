package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type diagnosticState struct {
	failed     bool
	last       time.Time
	suppressed int
}

// Diagnostics emits bounded, allowlisted metadata. It never renders err.Error().
type Diagnostics struct {
	mu       sync.Mutex
	logger   *slog.Logger
	cooldown time.Duration
	now      func() time.Time
	states   map[string]diagnosticState
}

func NewDiagnostics(logger *slog.Logger, cooldown time.Duration) *Diagnostics {
	if logger == nil {
		logger = slog.Default()
	}
	if cooldown <= 0 {
		cooldown = 10 * time.Second
	}
	return &Diagnostics{logger: logger, cooldown: cooldown, now: time.Now, states: make(map[string]diagnosticState)}
}

var diagnosticTopic = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,249}$`)
var diagnosticSQLState = regexp.MustCompile(`^[A-Z0-9]{5}$`)

func (d *Diagnostics) Report(kind, topic string, partition int32, offset int64, err error) {
	switch kind {
	case "startup", "storage", "quarantine", "progress":
	default:
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	state := d.states[kind]
	now := d.now()
	if err == nil && !state.failed {
		return
	}
	if err != nil && state.failed && now.Sub(state.last) < d.cooldown {
		state.suppressed++
		d.states[kind] = state
		return
	}
	attrs := []any{"operation", kind, "partition", partition, "offset", offset, "suppressed", state.suppressed}
	if diagnosticTopic.MatchString(topic) {
		attrs = append(attrs, "topic", topic)
	}
	if err == nil {
		d.logger.Info("consumer operation recovered", attrs...)
		d.states[kind] = diagnosticState{}
		return
	}
	class := "operation"
	sqlstate := ""
	var pg *pgconn.PgError
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		class = "deadline"
	case errors.Is(err, context.Canceled):
		class = "canceled"
	case errors.As(err, &pg):
		class = "database"
		if diagnosticSQLState.MatchString(pg.Code) {
			sqlstate = pg.Code
		}
	case errors.As(err, &network):
		class = "network"
	}
	attrs = append(attrs, "error_class", class)
	if sqlstate != "" {
		attrs = append(attrs, "sqlstate", sqlstate)
	}
	d.logger.Warn("consumer operation failed", attrs...)
	d.states[kind] = diagnosticState{failed: true, last: now}
}
