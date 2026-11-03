CREATE SCHEMA IF NOT EXISTS ingestkit;
CREATE TABLE ingestkit.events (
 tenant_id text NOT NULL CHECK (octet_length(tenant_id) BETWEEN 1 AND 128),
 event_id uuid NOT NULL,
 event_type text NOT NULL CHECK (octet_length(event_type) BETWEEN 1 AND 128),
 occurred_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 persisted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object'),
 PRIMARY KEY (tenant_id,event_id)
);
CREATE INDEX events_received_at ON ingestkit.events (received_at);
