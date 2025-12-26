# Migration Guide - GoREST Blog Plugin

This guide explains how the GoREST 0.4 migration system works within the blog plugin and how to create custom migrations.

## Table of Contents

1. [Understanding Migrations](#understanding-migrations)
2. [Migration File Structure](#migration-file-structure)
3. [How Migrations Work](#how-migrations-work)
4. [Creating New Migrations](#creating-new-migrations)
5. [Migration Commands](#migration-commands)
6. [Troubleshooting](#troubleshooting)
7. [Best Practices](#best-practices)

## Understanding Migrations

### What are Migrations?

Migrations are versioned database schema changes that allow you to:
- Define your database schema in code
- Apply changes incrementally
- Rollback changes if needed
- Share schema across teams
- Deploy safely to production

### Why Use Migrations?

Traditional approach (schema.sql):
```sql
-- schema.sql
DROP TABLE IF EXISTS posts CASCADE;
CREATE TABLE posts (...);
```

Problems:
- No version control
- Can't rollback
- Destructive on re-run
- No deployment tracking

Migration approach:
```sql
-- 20250121000001_create_posts.up.sql
CREATE TABLE posts (...);

-- 20250121000001_create_posts.down.sql
DROP TABLE posts CASCADE;
```

Benefits:
- Versioned and tracked
- Reversible
- Safe to re-run
- Deployment history

## Migration File Structure

### Naming Convention

```
{timestamp}_{description}.{direction}[.{dialect}].sql
```

Example:
```
20250121143022_create_posts_table.up.postgres.sql
20250121143022_create_posts_table.down.postgres.sql
```

**Components:**
- `20250121143022`: Timestamp (YYYYMMDDHHMMSS) - determines order
- `create_posts_table`: Description (snake_case)
- `up`: Apply migration (create, alter)
- `down`: Revert migration (drop, undo changes)
- `postgres`: Optional dialect (postgres, mysql, sqlite)

### Multi-Dialect Support

The migration system supports database-specific SQL:

```
migrations/
├── 20250121000001_create_posts.up.postgres.sql   # PostgreSQL version
├── 20250121000001_create_posts.up.mysql.sql      # MySQL version
├── 20250121000001_create_posts.up.sqlite.sql     # SQLite version
├── 20250121000001_create_posts.down.postgres.sql
├── 20250121000001_create_posts.down.mysql.sql
└── 20250121000001_create_posts.down.sqlite.sql
```

### Blog Plugin Migrations

The blog plugin includes these migrations:

**1. Create Posts Table** (`20250121000001_create_posts_table.*`)
```sql
-- Creates post_status enum
-- Creates post table with columns:
--   id, user_id, slug, status, title, content,
--   published_at, updated_at, created_at
-- Creates indexes: title, status, user_id, slug
```

**2. Create Comments Table** (`20250121000002_create_comments_table.*`)
```sql
-- Creates comment table with columns:
--   id, user_id, post_id, parent_id, content,
--   updated_at, created_at
-- Creates indexes: user_id, post_id, parent_id
-- Supports hierarchical comments via parent_id self-reference
```

**3. Create Likes Table** (`20250121000003_create_likes_table.*`)
```sql
-- Creates polymorphic likes table:
--   id, liker_id, liked_id, likeable, likeable_id,
--   liked_at, updated_at, created_at
-- Creates indexes and unique constraint
-- Supports liking posts and comments
```

## How Migrations Work

### Dependency Resolution

The blog plugin declares a dependency on the auth plugin:

```go
func (p *BlogPlugin) MigrationDependencies() []string {
    return []string{"auth"}
}
```

**Execution Order:**
1. Auth plugin migrations (creates users table)
2. Blog plugin migrations (uses users table for foreign keys)

### Migration Tracking

GoREST tracks applied migrations in a `schema_migrations` table:

```sql
CREATE TABLE schema_migrations (
    version TEXT NOT NULL,
    source TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL,
    execution_time_ms INTEGER,
    PRIMARY KEY (version, source)
);
```

**Fields:**
- `version`: Migration timestamp
- `source`: Plugin name (e.g., "blog", "auth")
- `checksum`: SHA256 of SQL to detect changes
- `applied_at`: When migration was run
- `execution_time_ms`: How long it took

### Automatic vs Manual Migrations

**Automatic (Default):**
```yaml
# gorest.yaml
migrations:
  enabled: true
  auto_migrate: true  # Runs on startup
```

On startup:
1. GoREST discovers all plugin migrations
2. Resolves dependencies (auth before blog)
3. Checks which migrations are pending
4. Applies pending migrations in order
5. Records in schema_migrations

**Manual:**
```yaml
migrations:
  enabled: true
  auto_migrate: false  # Manual control
```

```bash
gorest migrate up       # Apply all pending
gorest migrate status   # Check what's applied
```

### Transaction Safety

Each migration runs in a transaction:

```go
tx.Begin()
tx.Exec(migration.UpSQL)
tx.Commit()  // On success

// On error:
tx.Rollback()
RecordFailedMigration()
```

If a migration fails:
- Transaction is rolled back
- Database state is unchanged
- Error is logged with details
- Application startup can fail (safe by default)

## Creating New Migrations

### Step 1: Generate Timestamp

```bash
date +%Y%m%d%H%M%S
# Output: 20250121153045
```

Or use online tool: [Epoch Converter](https://www.epochconverter.com/)

### Step 2: Create Migration Files

**Example: Add tags to posts**

Create `migrations/20250121153045_add_tags_to_posts.up.postgres.sql`:
```sql
-- Add tags column to posts
ALTER TABLE post ADD COLUMN tags TEXT[];

-- Create index for tag searches
CREATE INDEX idx_post_tags ON post USING GIN (tags);
```

Create `migrations/20250121153045_add_tags_to_posts.down.postgres.sql`:
```sql
-- Remove tags index
DROP INDEX IF EXISTS idx_post_tags;

-- Remove tags column
ALTER TABLE post DROP COLUMN IF EXISTS tags;
```

### Step 3: Test Locally

```bash
# Check status
gorest migrate status

# Apply migration
gorest migrate up

# Verify schema
psql -d mydb -c "\d post"

# Test rollback
gorest migrate down

# Verify rollback worked
psql -d mydb -c "\d post"

# Re-apply
gorest migrate up
```

### Step 4: Commit to Version Control

```bash
git add migrations/20250121153045_add_tags_to_posts.*
git commit -m "feat: add tags support to posts"
```

## Migration Commands

### Check Status

```bash
gorest migrate status
```

Output:
```
Migration Status:
[auth] 20250120000001_create_users_table - applied (2025-01-20 14:30:22)
[blog] 20250121000001_create_posts_table - applied (2025-01-21 10:15:30)
[blog] 20250121000002_create_comments_table - applied (2025-01-21 10:15:31)
[blog] 20250121000003_create_likes_table - applied (2025-01-21 10:15:32)
[blog] 20250121153045_add_tags_to_posts - pending
```

### Apply All Pending

```bash
gorest migrate up
```

Output:
```
Applying migration [blog] 20250121153045_add_tags_to_posts...
✓ Applied [blog] 20250121153045_add_tags_to_posts (45ms)
```

### Apply One Migration

```bash
gorest migrate up-one
```

### Apply Specific Plugin

```bash
gorest migrate up --source blog
```

### Rollback Last Migration

```bash
gorest migrate down
```

Output:
```
Reverting migration [blog] 20250121153045_add_tags_to_posts...
✓ Reverted [blog] 20250121153045_add_tags_to_posts
```

### Rollback to Version

```bash
gorest migrate down-to 20250121000002
```

Rolls back all migrations after version `20250121000002`.

### Dry Run

```bash
gorest migrate up --dry-run
```

Shows what would be applied without executing.

### Validate Migrations

```bash
gorest migrate validate
```

Checks:
- Migration file naming
- Paired up/down files
- Checksum integrity
- Dependency cycles

### Force Mark as Applied

```bash
gorest migrate force 20250121153045 blog
```

**⚠️ WARNING**: Only use if migration was manually applied to database.

## Troubleshooting

### Migration Failed - Database in Dirty State

**Error:**
```
Error: database is in dirty state: migration 20250121153045 failed
```

**Cause:** Previous migration failed mid-execution.

**Solution:**
1. Check the migration SQL for errors
2. Manually verify database state
3. Fix issues in database
4. Mark migration as applied or fix and retry:

```bash
# Option 1: Fix database manually, then force
gorest migrate force 20250121153045 blog

# Option 2: Fix migration file, then retry
gorest migrate up
```

### Checksum Mismatch

**Error:**
```
Error: migration checksum mismatch for 20250121000001_create_posts
Expected: abc123...
Got: def456...
```

**Cause:** Migration file was modified after being applied.

**Solution:**
- **Never modify applied migrations** in production
- Create a new migration to make changes

If you're in development and need to reset:
```bash
# Rollback
gorest migrate down-to 0

# Modify migration file
vi migrations/20250121000001_create_posts.up.postgres.sql

# Re-apply
gorest migrate up
```

### Foreign Key Constraint Violation

**Error:**
```
Error: relation "users" does not exist
```

**Cause:** Migration dependency not met (auth plugin not enabled).

**Solution:**
```yaml
# gorest.yaml
plugins:
  - name: auth    # Required dependency
    enabled: true
  - name: blog
    enabled: true
```

### Migration Not Found

**Error:**
```
Error: migration not found: 20250121153045
```

**Cause:** Migration files not embedded or not in correct directory.

**Solution:**
Ensure migrations are embedded:
```go
//go:embed migrations/*.sql
var migrationFiles embed.FS
```

## Best Practices

### 1. Never Modify Applied Migrations

❌ **Bad:**
```bash
# Edit existing migration
vi migrations/20250121000001_create_posts.up.sql
```

✅ **Good:**
```bash
# Create new migration
vi migrations/20250121153050_add_post_views_column.up.sql
```

### 2. Always Provide Down Migrations

❌ **Bad:**
```sql
-- Only up migration, no down
```

✅ **Good:**
```sql
-- up: migrations/20250121153045_add_tags.up.sql
ALTER TABLE post ADD COLUMN tags TEXT[];

-- down: migrations/20250121153045_add_tags.down.sql
ALTER TABLE post DROP COLUMN tags;
```

### 3. Test Rollbacks

```bash
# Test cycle
gorest migrate up
gorest migrate down
gorest migrate up  # Should work identically
```

### 4. Keep Migrations Small

❌ **Bad:**
```sql
-- One huge migration
CREATE TABLE posts (...);
CREATE TABLE comments (...);
CREATE TABLE likes (...);
ALTER TABLE posts ADD COLUMN tags TEXT[];
-- 100 more lines...
```

✅ **Good:**
```sql
-- 20250121000001: Create posts
-- 20250121000002: Create comments
-- 20250121000003: Create likes
-- 20250121153045: Add tags
```

### 5. Use Descriptive Names

❌ **Bad:**
```
20250121153045_migration.up.sql
20250121153046_update.up.sql
```

✅ **Good:**
```
20250121153045_add_tags_to_posts.up.sql
20250121153046_add_published_index.up.sql
```

### 6. Handle Data Migrations Safely

When adding NOT NULL columns:

❌ **Bad:**
```sql
ALTER TABLE post ADD COLUMN category TEXT NOT NULL;
```

✅ **Good:**
```sql
-- Step 1: Add nullable column
ALTER TABLE post ADD COLUMN category TEXT;

-- Step 2: Populate with default
UPDATE post SET category = 'uncategorized' WHERE category IS NULL;

-- Step 3: Make NOT NULL
ALTER TABLE post ALTER COLUMN category SET NOT NULL;
```

### 7. Document Complex Migrations

```sql
-- Migration: Add full-text search support
-- Author: John Doe
-- Date: 2025-01-21
-- Reason: Enable fast post searching
--
-- This migration:
-- 1. Adds tsvector column for search
-- 2. Creates GIN index for performance
-- 3. Sets up trigger to auto-update search column

ALTER TABLE post ADD COLUMN search_vector tsvector;

CREATE INDEX idx_post_search ON post USING GIN (search_vector);

CREATE FUNCTION post_search_trigger() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector('english',
    coalesce(NEW.title, '') || ' ' || coalesce(NEW.content, '')
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_post_search
  BEFORE INSERT OR UPDATE ON post
  FOR EACH ROW EXECUTE FUNCTION post_search_trigger();
```

### 8. Coordinate with Team

Before deploying migrations:
1. Review with team
2. Test on staging environment
3. Plan rollback strategy
4. Communicate deployment window
5. Have database backup ready

## Summary

The GoREST migration system provides:
- ✅ Version-controlled schema
- ✅ Automatic dependency resolution
- ✅ Transaction safety
- ✅ Multi-dialect support
- ✅ Rollback capability
- ✅ Checksum verification
- ✅ Production-ready deployment

Follow this guide to safely manage your database schema as your blog plugin evolves!
