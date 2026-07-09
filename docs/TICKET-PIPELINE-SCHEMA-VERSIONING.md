# Ticket: Implement Warehouse Schema Versioning

**Priority:** Medium (post-MVP)  
**Assignee:** Pipeline team  
**Created:** 2026-07-09  
**Status:** Open

## Summary

Add a `schema_version` table to the cetus warehouse (cetus-marketdata-pipeline) to track the schema version. Downstream consumers (cetus-marketdata-scanner) will read this table at startup to detect breaking changes and fail fast with a clear error instead of silent data corruption.

## Motivation

The warehouse schema is a **contract** owned by the pipeline. Downstream consumers currently assume the schema is stable, but there's no mechanism to detect breaking changes (column renames, type changes, view redefinitions). As the pipeline evolves, we need a way to:

1. **Detect incompatibility early** — fail at startup with a clear error, not mid-scan with cryptic SQL errors
2. **Coordinate upgrades** — consumers can check "do I support this schema version?" before reading
3. **Document schema evolution** — the version number becomes a reference point for changelogs

## Current State (Scanner-Side)

The scanner already has a forward-compatible check in `internal/store/store.go`:

```go
func (s *Store) detectSchemaVersion(ctx context.Context) int {
    // Returns 0 if schema_version table is absent (pre-versioning warehouse)
}

func (s *Store) CheckSchema() error {
    if s.schema == 0 {
        return nil // unversioned warehouse — assume compatibility (MVP phase)
    }
    if s.schema < MinSchemaVersion {
        return fmt.Errorf("warehouse schema v%d is too old ...", s.schema)
    }
    if s.schema > MaxSchemaVersion {
        return fmt.Errorf("warehouse schema v%d is too new ...", s.schema)
    }
    return nil
}
```

The scanner logs a warning for unversioned warehouses (version 0) and continues. Once the pipeline implements versioning, the scanner will automatically detect and validate the version.

## Implementation (Pipeline-Side)

### 1. Create the `schema_version` table

Add to the pipeline's schema initialization (e.g., `internal/db/schema.go` or equivalent):

```sql
CREATE TABLE IF NOT EXISTS schema_version (
    id INTEGER PRIMARY KEY,
    version INTEGER NOT NULL,
    applied_at INTEGER NOT NULL,  -- Unix seconds (UTC)
    description TEXT
);
```

### 2. Stamp the initial version

On first ingestion (or a one-time migration), insert the current schema version:

```sql
INSERT INTO schema_version (id, version, applied_at, description)
VALUES (1, 1, unixepoch(), 'Initial versioned schema — published_bars, clean_bars, adjusted_bars, index_membership, symbol_pipeline_state');
```

**Current version = 1** (the schema as of 2026-07-09).

### 3. Bump the version on breaking changes

When the pipeline makes a breaking schema change (column rename, type change, view redefinition):

1. Increment the `version` number
2. Insert a new row with the new version and a description
3. Update the scanner's `MaxSchemaVersion` to match (if the scanner supports it)
4. Document the change in the pipeline's `CHANGELOG.md`

Example:

```sql
INSERT INTO schema_version (id, version, applied_at, description)
VALUES (2, 2, unixepoch(), 'Add split_adjusted_at column to published_bars');
```

### 4. Non-breaking changes

Additive changes (new columns, new tables) do **not** require a version bump. Downstream consumers that don't use the new columns are unaffected. Document additive changes in the `DATA_DICTIONARY.md` changelog.

## Scanner-Side Updates

When the pipeline implements versioning:

1. **Scanner reads the version** — already implemented in `internal/store/store.go`
2. **Scanner validates compatibility** — already implemented in `CheckSchema()`
3. **Update `MaxSchemaVersion`** — when the pipeline bumps to v2, update the scanner's `MaxSchemaVersion` to 2 (if the scanner supports v2)

## Acceptance Criteria

- [ ] `schema_version` table exists in the warehouse after ingestion
- [ ] Initial version = 1 is stamped on first run (or migration)
- [ ] Version is bumped on breaking schema changes
- [ ] `DATA_DICTIONARY.md` documents the versioning policy
- [ ] Scanner logs the warehouse version at startup (already implemented)
- [ ] Scanner fails fast if the version is outside the supported range (already implemented)

## Testing

1. **Fresh warehouse** — run the pipeline, verify `schema_version` has version 1
2. **Scanner startup** — run the scanner, verify it logs "warehouse schema version 1"
3. **Breaking change** — bump the pipeline to v2, verify the scanner (with `MaxSchemaVersion=1`) fails with a clear error
4. **Upgrade scanner** — update the scanner to `MaxSchemaVersion=2`, verify it accepts v2

## References

- Scanner implementation: `internal/store/store.go` (detectSchemaVersion, CheckSchema)
- Scanner constants: `MinSchemaVersion=1`, `MaxSchemaVersion=1`
- Data dictionary: `../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`
- Downstream contract: `../cetus-marketdata-pipeline/docs/DOWNSTREAM.md`
