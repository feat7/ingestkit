# Schema Versioning & Migration Strategy

This document outlines IngestKit's approach to schema evolution and version management.

## Overview

IngestKit tracks schema versions but currently uses a **simple forward-only migration strategy** suitable for event streaming systems.

**Current Schema Version**: `1.0` (defined in `schema/events.yaml`)

## Versioning Principles

### 1. Forward Compatibility

New schema changes must be **forward compatible** - older consumers can still process new events by ignoring unknown fields.

### 2. Backward Compatibility (Best Effort)

Schema changes should strive for backward compatibility where possible, but **at-least-once delivery** means old events may still arrive after schema updates.

### 3. Version Tracking

Every event envelope includes:
- `schema_version` field (`"v1"` currently)
- Allows future schema-aware routing or transformation

## Safe Schema Changes

### ✅ Always Safe

These changes won't break existing events or consumers:

1. **Adding optional fields**
   ```yaml
   # Before
   fields:
     user_id:
       type: string
       required: true

   # After (safe)
   fields:
     user_id:
       type: string
       required: true
     age:  # New optional field
       type: integer
       required: false
   ```

2. **Adding new event types**
   ```yaml
   # Safe to add new events without affecting existing ones
   events:
     user_signup: {...}  # Existing
     user_login: {...}   # New event type
   ```

3. **Expanding enum values**
   ```yaml
   # Before
   values: [web, mobile]

   # After (safe - new option)
   values: [web, mobile, api]
   ```

4. **Widening field types** (with caution)
   - `integer` → `decimal` (safe)
   - Small string → larger string (safe)

### ⚠️  Potentially Breaking

These changes require careful migration:

1. **Making fields required**
   - Old events without the field will fail validation
   - **Migration**: Add field as optional first, backfill data, then make required

2. **Removing fields**
   - Consumers may reference missing fields
   - **Migration**: Mark as deprecated, wait for consumers to update, then remove

3. **Renaming fields**
   - Breaks all consumers
   - **Migration**: Add new field, deprecate old, dual-write, migrate consumers, remove old

4. **Changing field types**
   - May cause parsing errors
   - **Migration**: Add new field with new type, migrate data, remove old

5. **Narrowing enum values**
   - Events with removed values will fail validation
   - **Migration**: Not recommended - add new field instead

### ❌ Never Safe

Avoid these changes:

1. **Removing event types** - breaks consumers expecting that type
2. **Changing primary key fields** - breaks database integrity
3. **Changing field semantics** - same name, different meaning

## Migration Workflow

### Step 1: Update Schema

Edit `schema/events.yaml`:

```yaml
version: "1.1"  # Increment version

events:
  user_signup:
    fields:
      # ... existing fields ...

      # New optional field (forward compatible)
      referral_code:
        type: string
        required: false
        description: User referral code (added v1.1)
```

### Step 2: Regenerate Code

```bash
make generate  # Regenerates models, storage, consumer
make build     # Rebuild binaries
```

### Step 3: Database Migration

For new fields, database schema updates are needed:

```sql
-- Add new column (nullable for backward compatibility)
ALTER TABLE events_user_signup
ADD COLUMN referral_code VARCHAR(255);

-- Add index if needed
CREATE INDEX idx_user_signup_referral
ON events_user_signup(referral_code)
WHERE referral_code IS NOT NULL;
```

### Step 4: Deploy

1. **Deploy Consumer** first (can handle new events)
2. **Deploy API** second (starts sending new fields)

This order ensures zero data loss.

### Step 5: Backfill (if needed)

For new required fields, backfill existing events:

```sql
UPDATE events_user_signup
SET referral_code = 'LEGACY'
WHERE referral_code IS NULL
  AND created_at < '2025-01-01';
```

## Version Compatibility Matrix

| Schema Version | API Compatible | Consumer Compatible | Notes |
|----------------|----------------|---------------------|-------|
| 1.0            | ≥ 1.0          | ≥ 1.0              | Initial version |
| 1.1            | ≥ 1.0          | ≥ 1.0              | Added optional fields only |

## Future Enhancements

### Schema Registry

For production systems, consider:

1. **Centralized Schema Registry**
   - Store all schema versions
   - Validate backward/forward compatibility automatically
   - Tools: Confluent Schema Registry, AWS Glue Schema Registry

2. **Automated Migration Generation**
   - Generate SQL ALTER statements from schema diffs
   - Generate backfill scripts
   - Validate migration safety

3. **Multi-Version Support**
   - Support multiple schema versions simultaneously
   - Transform events between versions
   - Gradual migration of old data

### Protobuf/Avro

Consider switching from JSON to schema-aware formats:

**Protobuf**:
```protobuf
message UserSignup {
  option (schema_version) = "1.1";

  string user_id = 1;
  string email = 2;
  string referral_code = 3 [optional];  // New field
}
```

**Benefits**:
- Built-in backward/forward compatibility
- Smaller payload size
- Faster serialization
- Compile-time type safety

## Breaking Change Process

If a breaking change is absolutely necessary:

### 1. Dual-Write Phase

API writes to both old and new format:

```go
// Write to old table
writer.WriteUserSignupV1(eventV1)

// Write to new table
writer.WriteUserSignupV2(eventV2)
```

### 2. Dual-Read Phase

Consumer reads from both:

```go
// Try new format first
events := reader.ReadV2(query)
if len(events) == 0 {
    // Fall back to old format
    events = reader.ReadV1(query)
}
```

### 3. Migration Phase

Backfill old events to new format:

```bash
./scripts/migrate-v1-to-v2.sh --batch-size 10000
```

### 4. Deprecation Phase

Stop writing to old format, log warnings for old readers:

```go
if schemaVersion == "v1" {
    log.Warn().Msg("Reading deprecated schema v1, please upgrade")
}
```

### 5. Removal Phase

After sufficient grace period (e.g., 90 days):
- Drop old tables
- Remove old code
- Update documentation

## Best Practices

1. **Version Everything**
   - Schema files: `events.yaml`
   - Database migrations: `001_initial.sql`, `002_add_referral.sql`
   - Code: Git tags for releases

2. **Document Changes**
   - Update CHANGELOG.md
   - Add migration notes
   - Update API documentation

3. **Test Migrations**
   - Test with real production data (anonymized)
   - Verify no data loss
   - Measure performance impact

4. **Monitor Versions**
   - Track schema_version in events
   - Alert on deprecated versions
   - Dashboard for version distribution

5. **Graceful Degradation**
   - Handle unknown fields gracefully
   - Log warnings, don't fail
   - Provide fallback values

## Example: Adding Required Field

**Requirement**: Add `country_code` as required field to `user_signup`.

**Safe Migration**:

```yaml
# Step 1: Add as optional (schema v1.1)
country_code:
  type: string
  required: false
  description: User country code (ISO 3166-1 alpha-2)

# Step 2: Deploy, collect data, backfill
# (wait 30 days)

# Step 3: Make required (schema v1.2)
country_code:
  type: string
  required: true
  description: User country code (ISO 3166-1 alpha-2)
```

**SQL Migration**:

```sql
-- Step 1: Add column (v1.1)
ALTER TABLE events_user_signup
ADD COLUMN country_code VARCHAR(2);

-- Step 2: Backfill (after 30 days)
UPDATE events_user_signup
SET country_code = COALESCE(
    metadata->>'country',  -- Try to extract from existing data
    'US'                    -- Default fallback
)
WHERE country_code IS NULL;

-- Step 3: Add NOT NULL constraint (v1.2)
ALTER TABLE events_user_signup
ALTER COLUMN country_code SET NOT NULL;
```

## Schema Version Validation

The schema parser validates version format:

```go
// internal/schema/parser.go
func validateSchemaVersion(version string) error {
    // Version must be in format: "major.minor" or "major.minor.patch"
    if !regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`).MatchString(version) {
        return fmt.Errorf("invalid schema version format: %s", version)
    }
    return nil
}
```

## References

- [Confluent Schema Evolution](https://docs.confluent.io/platform/current/schema-registry/avro.html#schema-evolution)
- [Event Versioning Pattern](https://domaincentric.net/blog/event-versioning)
- [Database Migrations Best Practices](https://www.postgresql.org/docs/current/ddl-alter.html)

---

**Last Updated**: 2025-11-15
**Current Schema Version**: 1.0
**Next Planned Version**: 1.1 (optional referral fields)
