# Automated Schema Migrations with Atlas

**Prisma-style automatic migration SQL generation** for IngestKit.

Last updated: 2025-11-15

---

## Overview

IngestKit uses **Atlas** to automatically generate migration SQL by comparing your current database schema with your desired schema (defined in `schema/events.yaml`).

**Just like Prisma Migrate:**
- Edit your YAML schema
- Run one command
- Migration SQL is generated automatically
- Review, test, and apply

---

## Quick Start

### 1. Edit Schema

```bash
vim schema/events.yaml
```

Add a new field to an event:

```yaml
user_signup:
  fields:
    referrer:
      type: string
      required: false
      description: "Referrer URL where user came from"
```

### 2. Auto-Generate Migration

```bash
make migrate-auto NAME=add_user_referrer
```

This command:
1. Regenerates Go code from schema
2. Compares current DB with new schema
3. **Automatically generates migration SQL** (both `.up.sql` and `.down.sql`)
4. Shows you the generated files

Output:
```
Auto-generating migration: add_user_referrer

Step 1/3: Regenerating code from schema...
✓ Code generated

Step 2/3: Computing schema diff...
✓ Migration generated

Step 3/3: Review migration files:
  migrations/20251115104512_add_user_referrer.up.sql (206B)
  migrations/20251115104512_add_user_referrer.down.sql (112B)

✓ Migration ready!

Next steps:
  1. Review migration files in migrations/ directory
  2. Test: make db-migrate-up
  3. Verify: Send test events
  4. Rollback if needed: make db-migrate-down
  5. Deploy: make docker-reload
```

### 3. Review Generated SQL

```bash
cat migrations/*add_user_referrer.up.sql
```

Atlas generated:
```sql
-- modify "events_user_signup" table
ALTER TABLE "public"."events_user_signup" ADD COLUMN "referrer" character varying(255) NULL;
```

### 4. Apply Migration

```bash
make db-migrate-up
```

### 5. Test It

```bash
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "email": "test@example.com",
    "referrer": "https://google.com"
  }'
```

### 6. Deploy with Zero Downtime

```bash
make docker-reload
```

---

## Complete Workflow

### Workflow Comparison

**Manual migrations (old way):**
```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Regenerate code
make generate

# 3. Create empty migration files
make db-migrate-create NAME=add_field

# 4. Manually write ALTER TABLE statements
vim migrations/NNNN_add_field.up.sql   # ❌ Manual SQL writing
vim migrations/NNNN_add_field.down.sql # ❌ Manual SQL writing

# 5. Apply
make db-migrate-up
```

**Automated migrations (new way with Atlas):**
```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Auto-generate everything
make migrate-auto NAME=add_field  # ✅ SQL generated automatically!

# 3. Review and apply
make db-migrate-up
```

---

## How It Works

Atlas compares two states:

**Before (current database):**
```sql
CREATE TABLE events_user_signup (
    user_id VARCHAR(255),
    email VARCHAR(255)
);
```

**After (from schema/events.yaml → generated/sql/schema.sql):**
```sql
CREATE TABLE events_user_signup (
    user_id VARCHAR(255),
    email VARCHAR(255),
    referrer VARCHAR(255)  -- NEW FIELD
);
```

**Atlas generates:**
```sql
ALTER TABLE events_user_signup ADD COLUMN referrer VARCHAR(255);
```

---

## Common Scenarios

### Adding a Field

**Edit schema:**
```yaml
user_signup:
  fields:
    country:
      type: string
      required: false
```

**Generate migration:**
```bash
make migrate-auto NAME=add_user_country
```

**Atlas generates:**
```sql
-- up
ALTER TABLE events_user_signup ADD COLUMN country VARCHAR(255);

-- down
ALTER TABLE events_user_signup DROP COLUMN country;
```

### Adding an Index

**Edit schema:**
```yaml
user_signup:
  fields:
    email:
      type: string
      required: true
      indexed: true  # Add this
```

**Generate migration:**
```bash
make migrate-auto NAME=add_email_index
```

**Atlas generates:**
```sql
-- up
CREATE INDEX idx_user_signup_email ON events_user_signup(email);

-- down
DROP INDEX idx_user_signup_email;
```

### Adding a New Event Type

**Edit schema:**
```yaml
user_login:
  description: "User login event"
  fields:
    user_id:
      type: string
      required: true
```

**Generate migration:**
```bash
make migrate-auto NAME=add_user_login_event
```

**Atlas generates:**
```sql
-- up
CREATE TABLE events_user_login (
    tenant_id VARCHAR(255) NOT NULL,
    event_id BIGSERIAL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (tenant_id, event_id)
) PARTITION BY LIST (tenant_id);

CREATE INDEX idx_user_login_user_id ON events_user_login(user_id);
CREATE INDEX idx_user_login_timestamp ON events_user_login(timestamp);

-- down
DROP TABLE events_user_login;
```

### Changing Field Type (Advanced)

**IMPORTANT:** Type changes require careful handling. Atlas will generate destructive migrations.

**Edit schema:**
```yaml
user_signup:
  fields:
    user_id:
      type: integer  # Changed from string
```

**Generate migration:**
```bash
make migrate-auto NAME=change_user_id_to_int
```

**Review carefully!** Atlas may generate:
```sql
ALTER TABLE events_user_signup
  ALTER COLUMN user_id TYPE bigint USING user_id::bigint;
```

For production, you may need to manually edit this to:
```sql
-- Add new column
ALTER TABLE events_user_signup ADD COLUMN user_id_new BIGINT;

-- Migrate data
UPDATE events_user_signup
SET user_id_new = user_id::BIGINT
WHERE user_id ~ '^[0-9]+$';

-- Later: drop old column and rename (separate migration)
```

---

## Configuration

Atlas configuration is in `atlas.hcl`:

```hcl
env "local" {
  // Desired schema (auto-generated from YAML)
  src = "file://generated/sql/schema.sql"

  // Current database
  url = "postgres://ingestkit:ingestkit_dev@localhost:5433/ingestkit?sslmode=disable"

  // Dev database for schema diffing
  dev = "docker://postgres/18/dev"

  migration {
    dir = "file://migrations"
    format = golang-migrate  // Compatible with existing migrations!
  }
}
```

---

## Troubleshooting

### "Checksum file not found"

```bash
# Initialize Atlas checksums
atlas migrate hash --env local
```

### Migration Includes Unwanted Changes

Atlas compared DB with schema and found differences. Either:

**Option 1:** The schema is correct, DB is outdated
```bash
# Apply the migration
make db-migrate-up
```

**Option 2:** The migration is wrong
```bash
# Delete the bad migration files
rm migrations/*unwanted_name*

# Fix your schema
vim schema/events.yaml

# Regenerate
make migrate-auto NAME=correct_name
```

### "Dirty database version"

A migration failed mid-way:

```bash
# Check current version
make db-migrate-version

# Force to last known good version
make db-migrate-force VERSION=N

# Fix migration, then retry
make db-migrate-up
```

### Atlas Generated DROP Commands

Atlas tries to make DB match schema exactly. If your `generated/sql/schema.sql` is missing something (like DLQ tables), Atlas will try to drop it.

**Solution:** Review `.up.sql` and remove unwanted DROP statements:

```sql
-- Remove this if you want to keep the schema
DROP SCHEMA "ingestkit_meta" CASCADE;
```

Then update checksums:
```bash
atlas migrate hash --env local
```

---

## Best Practices

### 1. Always Review Generated SQL

```bash
# After running migrate-auto
cat migrations/*latest*.up.sql
cat migrations/*latest*.down.sql
```

Atlas is smart but not perfect. Check for:
- Unwanted DROP statements
- Destructive type changes
- Missing NOT NULL constraints with no defaults

### 2. Test Locally First

```bash
# Apply
make db-migrate-up

# Test with real events
curl -X POST http://localhost:8080/v1/events/...

# Rollback to test down migration
make db-migrate-down

# Re-apply
make db-migrate-up
```

### 3. Use Descriptive Names

```bash
# Good
make migrate-auto NAME=add_user_timezone_field
make migrate-auto NAME=create_user_login_event
make migrate-auto NAME=add_email_index

# Bad
make migrate-auto NAME=update
make migrate-auto NAME=schema_changes
make migrate-auto NAME=fix
```

### 4. Commit Schema + Migrations Together

```bash
git add schema/events.yaml
git add migrations/
git add generated/
git commit -m "Add user timezone tracking"
```

### 5. Update Checksums After Manual Edits

If you manually edit generated migrations:

```bash
atlas migrate hash --env local
```

---

## Manual Migration Fallback

You can still create manual migrations when needed:

```bash
# Create empty migration files
make db-migrate-create NAME=custom_migration

# Edit manually
vim migrations/NNNN_custom_migration.up.sql
vim migrations/NNNN_custom_migration.down.sql

# Update checksums
atlas migrate hash --env local

# Apply
make db-migrate-up
```

---

## Integration with Existing Tools

### Works with golang-migrate

Atlas generates migrations in `golang-migrate` format, so:

```bash
# All existing commands still work
make db-migrate-up
make db-migrate-down
make db-migrate-version
make db-migrate-force VERSION=N
```

### Works with Docker

```bash
# Zero-downtime deployment
make docker-reload
```

The Docker `migrate` service will automatically apply Atlas-generated migrations on startup.

---

## Summary

**Old Workflow (Manual):**
1. Edit schema
2. Generate code: `make generate`
3. Create migration: `make db-migrate-create`
4. **Manually write SQL** ❌
5. Apply: `make db-migrate-up`

**New Workflow (Automated with Atlas):**
1. Edit schema
2. **Auto-generate migration**: `make migrate-auto NAME=foo` ✅
3. Review SQL (already written!)
4. Apply: `make db-migrate-up`

**Benefits:**
- ✅ **Automatic SQL generation** - No manual ALTER TABLE writing
- ✅ **Fewer errors** - Atlas knows SQL schema syntax
- ✅ **Up and down migrations** - Both generated automatically
- ✅ **Compatible with existing tools** - Works with golang-migrate
- ✅ **Prisma-style DX** - Modern developer experience

**Commands:**
```bash
# Automated (recommended)
make migrate-auto NAME=description

# Manual (when needed)
make db-migrate-create NAME=description

# Apply
make db-migrate-up

# Rollback
make db-migrate-down

# Deploy
make docker-reload
```

---

## Related Documentation

- [Migrations Guide](migrations.md) - Manual migration workflow
- [Development Guide](development.md) - Local development
- [Docker Deployment](docker-deployment.md) - Zero-downtime deploys
- [Atlas Documentation](https://atlasgo.io) - Official Atlas docs

---

**Next:** Try it yourself! Add a field to your schema and run `make migrate-auto NAME=test`
