package service

import "sync"

// ProcessingHealth survives reader recreation. A successful dependency probe
// cannot clear an operation failure; that operation must actually succeed.
type ProcessingHealth struct {
	mu     sync.RWMutex
	failed map[string]bool
}

func NewProcessingHealth() *ProcessingHealth { return &ProcessingHealth{failed: map[string]bool{}} }
func (h *ProcessingHealth) Observe(operation string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		h.failed[operation] = true
	} else {
		delete(h.failed, operation)
	}
}
func (h *ProcessingHealth) Ready() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.failed) == 0
}
