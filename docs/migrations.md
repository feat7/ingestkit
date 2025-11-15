# Database Migrations Guide

Complete guide for managing database schema changes with golang-migrate.

Last updated: 2025-11-15

---

## Overview

IngestKit uses **golang-migrate** for database schema versioning and migrations. This enables:

- **Zero-downtime deployments** - Apply schema changes without stopping services
- **Version control** - Track schema changes in Git alongside code
- **Rollback support** - Revert schema changes if needed
- **Idempotent migrations** - Safe to run multiple times

---

## Quick Start

### Create a Migration

```bash
# Create new migration files
make db-migrate-create NAME=add_user_referrer_field

# This creates two files:
# migrations/000002_add_user_referrer_field.up.sql
# migrations/000002_add_user_referrer_field.down.sql
```

### Edit Migration Files

**Up migration** (migrations/000002_add_user_referrer_field.up.sql):
```sql
-- Add referrer field to events_user_signup
ALTER TABLE events_user_signup ADD COLUMN referrer VARCHAR(255);

-- Create index if needed
CREATE INDEX idx_user_signup_referrer ON events_user_signup(referrer);
```

**Down migration** (migrations/000002_add_user_referrer_field.down.sql):
```sql
-- Rollback: drop the referrer field
DROP INDEX IF EXISTS idx_user_signup_referrer;
ALTER TABLE events_user_signup DROP COLUMN IF EXISTS referrer;
```

### Apply Migrations

```bash
# Local development
make db-migrate-up

# Docker deployment (zero-downtime)
make docker-reload
```

---

## Complete Workflow

### 1. Edit Schema

```bash
vim schema/events.yaml
```

Add/remove fields from event definitions.

### 2. Regenerate Code

```bash
make generate
```

This regenerates Go models, storage writers, and SQL DDL.

### 3. Create Migration

```bash
make db-migrate-create NAME=descriptive_name
```

### 4. Write Migration SQL

Edit the `.up.sql` and `.down.sql` files:

**For adding columns:**
```sql
-- Up
ALTER TABLE events_user_signup ADD COLUMN IF NOT EXISTS new_field VARCHAR(255);

-- Down
ALTER TABLE events_user_signup DROP COLUMN IF EXISTS new_field;
```

**For adding indexes:**
```sql
-- Up
CREATE INDEX IF NOT EXISTS idx_table_field ON events_table(field);

-- Down
DROP INDEX IF EXISTS idx_table_field;
```

**For new tables:**
```sql
-- Up
CREATE TABLE IF NOT EXISTS events_new_event (
    tenant_id VARCHAR(255) NOT NULL,
    event_id BIGSERIAL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    field1 VARCHAR(255) NOT NULL,
    PRIMARY KEY (tenant_id, event_id)
) PARTITION BY LIST (tenant_id);

CREATE INDEX IF NOT EXISTS idx_new_event_field1 ON events_new_event(field1);

-- Down
DROP TABLE IF EXISTS events_new_event;
```

### 5. Test Locally

```bash
# Apply migration
make db-migrate-up

# Check version
make db-migrate-version

# Test with sample data
curl -X POST http://localhost:8080/v1/events/user_signup ...

# Rollback if needed
make db-migrate-down
```

### 6. Commit Changes

```bash
git add schema/events.yaml migrations/ generated/
git commit -m "Add user referrer tracking"
git push
```

### 7. Deploy with Zero Downtime

```bash
# For Docker deployment
make docker-reload
```

This will:
1. Generate code from schema
2. Build new binaries
3. Build Docker images
4. Run migrations automatically
5. Rolling update API and consumer (zero downtime)

---

## Migration Commands

### Local Development

```bash
# Create new migration
make db-migrate-create NAME=migration_name

# Apply all pending migrations
make db-migrate-up

# Rollback last migration
make db-migrate-down

# Check current version
make db-migrate-version

# Force specific version (for existing DBs)
make db-migrate-force VERSION=1
```

### Docker Deployment

Migrations run automatically on `docker-compose up`:

```bash
# Start stack (runs migrations first)
make docker-up

# Zero-downtime reload after schema changes
make docker-reload
```

---

## Migration Patterns

### Adding Optional Field

```sql
-- Up
ALTER TABLE events_user_signup
ADD COLUMN signup_device VARCHAR(255);

-- Down
ALTER TABLE events_user_signup
DROP COLUMN signup_device;
```

### Adding Required Field (with default)

```sql
-- Up
ALTER TABLE events_user_signup
ADD COLUMN country VARCHAR(255) NOT NULL DEFAULT 'US';

-- Later, remove default after backfill
ALTER TABLE events_user_signup
ALTER COLUMN country DROP DEFAULT;

-- Down
ALTER TABLE events_user_signup
DROP COLUMN country;
```

### Renaming Column (safe)

```sql
-- Up: Add new column, copy data, then drop old
ALTER TABLE events_user_signup ADD COLUMN email_address VARCHAR(255);
UPDATE events_user_signup SET email_address = email WHERE email_address IS NULL;
-- Don't drop old column yet! Keep it for rollback safety.

-- Down
ALTER TABLE events_user_signup DROP COLUMN email_address;
```

### Changing Column Type (risky)

```sql
-- Up: Create new column, migrate data, rename
ALTER TABLE events_user_signup ADD COLUMN user_id_new BIGINT;
UPDATE events_user_signup SET user_id_new = user_id::BIGINT;
ALTER TABLE events_user_signup DROP COLUMN user_id;
ALTER TABLE events_user_signup RENAME COLUMN user_id_new TO user_id;

-- Down: Reverse the process
ALTER TABLE events_user_signup ADD COLUMN user_id_old VARCHAR(255);
UPDATE events_user_signup SET user_id_old = user_id::VARCHAR;
ALTER TABLE events_user_signup DROP COLUMN user_id;
ALTER TABLE events_user_signup RENAME COLUMN user_id_old TO user_id;
```

### Adding Index (safe)

```sql
-- Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_signup_email
ON events_user_signup(email);

-- Down
DROP INDEX CONCURRENTLY IF EXISTS idx_user_signup_email;
```

---

## Best Practices

### 1. Keep Migrations Small

One logical change per migration:
- ✅ `add_user_referrer_field`
- ✅ `add_product_category_index`
- ❌ `update_schema_v2` (too vague)

### 2. Always Write Down Migrations

Even if you don't plan to rollback, down migrations:
- Document what the up migration does
- Enable testing rollbacks in staging
- Required for production safety

### 3. Test Rollbacks

```bash
# Apply migration
make db-migrate-up

# Test rollback
make db-migrate-down

# Re-apply
make db-migrate-up
```

### 4. Use IF NOT EXISTS / IF EXISTS

Makes migrations idempotent:
```sql
ALTER TABLE events_user_signup ADD COLUMN IF NOT EXISTS field VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_name ON table(field);
DROP INDEX IF EXISTS idx_name;
```

### 5. Handle Existing Data

When adding NOT NULL columns:
```sql
-- Add as nullable first
ALTER TABLE events_user_signup ADD COLUMN country VARCHAR(255);

-- Backfill existing rows
UPDATE events_user_signup SET country = 'US' WHERE country IS NULL;

-- Make NOT NULL
ALTER TABLE events_user_signup ALTER COLUMN country SET NOT NULL;
```

### 6. Use Concurrent Index Creation

For production databases with data:
```sql
CREATE INDEX CONCURRENTLY idx_name ON table(field);
```

This doesn't lock the table.

---

## Troubleshooting

### Migration Failed Mid-Way

```bash
# Check error
make db-migrate-version

# If stuck in dirty state
make db-migrate-force VERSION=N  # Last known good version

# Fix the migration file, then retry
make db-migrate-up
```

### Version Mismatch

```sql
-- Check schema_migrations table directly
docker exec ingestkit-postgres psql -U ingestkit -d ingestkit \
  -c "SELECT * FROM schema_migrations;"

-- Force specific version if needed
make db-migrate-force VERSION=2
```

### Existing Database Without Migrations

If you have an existing database and want to start using migrations:

```bash
# Mark initial schema as applied (don't run it)
make db-migrate-force VERSION=1

# Verify
make db-migrate-version  # Should show: 1

# New migrations will apply from version 2 onward
make db-migrate-create NAME=add_new_field
```

### Docker Migration Fails

```bash
# Check migration service logs
docker-compose logs migrate

# Manually run migration
docker-compose run --rm migrate \
  -path /migrations \
  -database "postgres://ingestkit:ingestkit_dev@postgres:5432/ingestkit?sslmode=disable" \
  up
```

---

## Migration File Structure

```
migrations/
├── 000001_initial_schema.up.sql       # Initial schema
├── 000001_initial_schema.down.sql     # Rollback initial
├── 000002_add_referrer_field.up.sql   # Add referrer
├── 000002_add_referrer_field.down.sql # Drop referrer
└── 000003_add_category_index.up.sql   # Add index
└── 000003_add_category_index.down.sql # Drop index
```

Migrations are applied in sequential order based on version number.

---

## Docker Integration

The `migrate` service in docker-compose.yml:

```yaml
migrate:
  image: migrate/migrate:latest
  command: >
    -path /migrations
    -database "postgres://user:pass@postgres:5432/db?sslmode=disable"
    up
  volumes:
    - ./migrations:/migrations
  depends_on:
    postgres:
      condition: service_healthy
  restart: "no"  # Run once and exit
```

API and consumer depend on migrate completing:

```yaml
api:
  depends_on:
    migrate:
      condition: service_completed_successfully
```

This ensures schema is up-to-date before services start.

---

## Related Documentation

- [Development Guide](development.md) - Local development workflow
- [Docker Deployment](docker-deployment.md) - Zero-downtime deployments
- [Schema Management](development.md#schema-management) - YAML schema editing
- [golang-migrate docs](https://github.com/golang-migrate/migrate) - Official documentation

---

## Summary

**Schema Change Workflow:**

```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Generate code
make generate

# 3. Create migration
make db-migrate-create NAME=descriptive_name

# 4. Write migration SQL
vim migrations/NNNN_descriptive_name.up.sql
vim migrations/NNNN_descriptive_name.down.sql

# 5. Test locally
make db-migrate-up

# 6. Commit
git add schema migrations generated
git commit -m "Add field"

# 7. Deploy (zero-downtime)
make docker-reload
```

**Zero downtime guaranteed** - migrations run before services start, and rolling updates ensure continuous availability.
