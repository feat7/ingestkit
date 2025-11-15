// Package storage provides database storage utilities including automatic partition management.
package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// PartitionManager handles automatic creation of table partitions.
// Partitions are created on-demand when a new tenant is encountered.
type PartitionManager struct {
	pool            *pgxpool.Pool
	createdMu       sync.RWMutex
	createdPartitions map[string]bool // track created partitions to avoid repeated checks
}

// NewPartitionManager creates a new partition manager.
func NewPartitionManager(pool *pgxpool.Pool) *PartitionManager {
	return &PartitionManager{
		pool:            pool,
		createdPartitions: make(map[string]bool),
	}
}

// EnsurePartition ensures a partition exists for the given table and tenant.
// Creates the partition if it doesn't exist.
// This is idempotent and safe to call multiple times.
func (pm *PartitionManager) EnsurePartition(ctx context.Context, tableName, tenantID string) error {
	partitionKey := fmt.Sprintf("%s_%s", tableName, tenantID)

	// Fast path: check if we already created this partition
	pm.createdMu.RLock()
	if pm.createdPartitions[partitionKey] {
		pm.createdMu.RUnlock()
		return nil
	}
	pm.createdMu.RUnlock()

	// Slow path: create partition if needed
	pm.createdMu.Lock()
	defer pm.createdMu.Unlock()

	// Double-check after acquiring write lock
	if pm.createdPartitions[partitionKey] {
		return nil
	}

	// Sanitize tenant_id for use in table name (replace hyphens with underscores)
	// PostgreSQL table names can't have hyphens
	sanitizedTenant := sanitizeTenantID(tenantID)
	partitionName := fmt.Sprintf("%s_%s", tableName, sanitizedTenant)

	// Create partition using CREATE TABLE IF NOT EXISTS
	// This is safe for concurrent execution
	createSQL := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN ('%s')",
		partitionName,
		tableName,
		tenantID, // Use original tenant_id for the partition value
	)

	_, err := pm.pool.Exec(ctx, createSQL)
	if err != nil {
		log.Error().
			Str("table", tableName).
			Str("tenant_id", tenantID).
			Str("partition", partitionName).
			Err(err).
			Msg("Failed to create partition")
		return fmt.Errorf("failed to create partition %s: %w", partitionName, err)
	}

	// Mark as created
	pm.createdPartitions[partitionKey] = true

	log.Info().
		Str("table", tableName).
		Str("tenant_id", tenantID).
		Str("partition", partitionName).
		Msg("Created partition")

	return nil
}

// sanitizeTenantID converts tenant_id to a valid PostgreSQL table name suffix.
// Replaces hyphens and other special characters with underscores.
func sanitizeTenantID(tenantID string) string {
	// Replace hyphens with underscores for table name
	result := make([]byte, len(tenantID))
	for i, c := range []byte(tenantID) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result[i] = c
		} else {
			result[i] = '_'
		}
	}
	return string(result)
}

// GetEventTableName returns the base table name for an event type.
// Event type "user_signup" -> "events_user_signup"
func GetEventTableName(eventType string) string {
	return fmt.Sprintf("events_%s", eventType)
}
