-- IngestKit Database Initialization
-- This script runs automatically when PostgreSQL container starts for the first time
-- Using PostgreSQL 18 with async I/O, UUID v7 support, and improved performance

-- Create extensions
-- Note: PostgreSQL 18 has built-in uuidv7() function for timestamp-ordered UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- Create schema for metadata
CREATE SCHEMA IF NOT EXISTS ingestkit_meta;

-- Schema versions table (for tracking migrations)
CREATE TABLE IF NOT EXISTS ingestkit_meta.schema_versions (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    description TEXT
);

-- Dead letter queue for failed events
CREATE TABLE IF NOT EXISTS ingestkit_meta.dead_letter_queue (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    error_message TEXT NOT NULL,
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_retry_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dlq_event_type ON ingestkit_meta.dead_letter_queue(event_type);
CREATE INDEX IF NOT EXISTS idx_dlq_tenant_id ON ingestkit_meta.dead_letter_queue(tenant_id);
CREATE INDEX IF NOT EXISTS idx_dlq_created_at ON ingestkit_meta.dead_letter_queue(created_at);
CREATE INDEX IF NOT EXISTS idx_dlq_resolved ON ingestkit_meta.dead_letter_queue(resolved_at) WHERE resolved_at IS NULL;

-- API keys table (for authentication)
CREATE TABLE IF NOT EXISTS ingestkit_meta.api_keys (
    id BIGSERIAL PRIMARY KEY,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    tenant_id VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON ingestkit_meta.api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON ingestkit_meta.api_keys(is_active) WHERE is_active = true;

-- Event metadata table (for tracking event types and their schemas)
CREATE TABLE IF NOT EXISTS ingestkit_meta.event_types (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL UNIQUE,
    schema_version VARCHAR(50) NOT NULL,
    table_name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ingestion stats table (for monitoring)
CREATE TABLE IF NOT EXISTS ingestkit_meta.ingestion_stats (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    hour TIMESTAMPTZ NOT NULL,  -- Rounded to hour for aggregation
    event_count BIGINT NOT NULL DEFAULT 0,
    bytes_ingested BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    UNIQUE(event_type, tenant_id, hour)
);

CREATE INDEX IF NOT EXISTS idx_stats_hour ON ingestkit_meta.ingestion_stats(hour);
CREATE INDEX IF NOT EXISTS idx_stats_event_type ON ingestkit_meta.ingestion_stats(event_type);
CREATE INDEX IF NOT EXISTS idx_stats_tenant ON ingestkit_meta.ingestion_stats(tenant_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION ingestkit_meta.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for event_types table
CREATE TRIGGER update_event_types_updated_at
    BEFORE UPDATE ON ingestkit_meta.event_types
    FOR EACH ROW
    EXECUTE FUNCTION ingestkit_meta.update_updated_at_column();

-- Insert initial schema version
INSERT INTO ingestkit_meta.schema_versions (version, description)
VALUES ('v1.0.0', 'Initial schema setup')
ON CONFLICT (version) DO NOTHING;

-- Grant permissions (if needed for specific roles)
-- GRANT ALL PRIVILEGES ON SCHEMA ingestkit_meta TO ingestkit;
-- GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA ingestkit_meta TO ingestkit;
-- GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA ingestkit_meta TO ingestkit;

-- Success message
DO $$
BEGIN
    RAISE NOTICE '✓ IngestKit database initialized successfully';
    RAISE NOTICE '✓ Schema version: v1.0.0';
    RAISE NOTICE '✓ Metadata tables created in schema: ingestkit_meta';
END $$;
