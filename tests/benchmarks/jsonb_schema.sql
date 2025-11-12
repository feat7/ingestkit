-- JSONB-Only Schema (for comparison)
-- All event data stored as JSON in a single table

CREATE TABLE IF NOT EXISTS events_jsonb (
    tenant_id VARCHAR(255) NOT NULL,
    event_id BIGSERIAL,
    event_type VARCHAR(255) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL,
    PRIMARY KEY (tenant_id, event_id)
) PARTITION BY LIST (tenant_id);

-- Indexes for common access patterns
CREATE INDEX IF NOT EXISTS idx_jsonb_event_type ON events_jsonb(event_type);
CREATE INDEX IF NOT EXISTS idx_jsonb_timestamp ON events_jsonb(timestamp);

-- GIN index for JSONB queries (this is the key performance feature for JSONB)
CREATE INDEX IF NOT EXISTS idx_jsonb_payload ON events_jsonb USING GIN (payload);

-- Create a default partition for testing (in production, you'd create per-tenant partitions)
CREATE TABLE IF NOT EXISTS events_jsonb_default PARTITION OF events_jsonb DEFAULT;
