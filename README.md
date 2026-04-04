# GoREST Blog Plugin

[![CI](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml/badge.svg)](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasbonnici/gorest-blog)](https://goreportcard.com/report/github.com/nicolasbonnici/gorest-blog)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A production-ready blog plugin for GoREST with multi-language support, RBAC, and metrics.

## Features

- **Multi-Language Posts**: Translate titles and content into multiple languages
- **Role-Based Access Control**: Reader, Moderator, and Writer roles
- **Post Metrics**: Track views, likes, and comments
- **Built-in Migrations**: Automatic database schema management
- **Multi-Database**: PostgreSQL, MySQL, and SQLite
- **RESTful API**: Full CRUD operations
- **Bidirectional Sync**: Import from and export to external platforms (dev.to)

## Requirements

- Go 1.25.1+
- GoREST 0.4+
- PostgreSQL, MySQL, or SQLite


## Development Environment

To set up your development environment:

```bash
make install
```

This will:
- Install Go dependencies
- Install development tools (golangci-lint)
- Set up git hooks (pre-commit linting and tests)

## Quick Start

```yaml
# gorest.yaml
database:
  url: "${DATABASE_URL}"

plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"

  - name: rbac
    enabled: true

  - name: translatable
    enabled: true
    config:
      allowed_types: ["post"]
      supported_locales: ["en", "fr", "es"]
      default_locale: "en"

  - name: metrics
    enabled: true

  - name: blog
    enabled: true

migrations:
  enabled: true
  auto_migrate: true
```

## RBAC Roles

| Role | Posts | Comments | Likes |
|------|-------|----------|-------|
| **reader** | View | Create, edit/delete own | Create, delete own |
| **moderator** | View, change status | Inherits reader + change status | Inherits reader |
| **writer** | Full access | Inherits moderator | Inherits moderator |

## API Usage

### Create Post

```bash
POST /posts
Authorization: Bearer <token>
```

```json
{
  "slug": "my-post",
  "status": "published",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "Post content in English..."
    },
    "fr": {
      "title": "Mon Article",
      "content": "Contenu en français..."
    }
  }
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "slug": "my-post",
  "status": "published",
  "publishedAt": "2025-02-13T10:00:00Z",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "Post content in English..."
    },
    "fr": {
      "title": "Mon Article",
      "content": "Contenu en français..."
    }
  },
  "metrics": {
    "views": 0,
    "likes": 0,
    "comments": 0
  }
}
```

### Update Post

```bash
PUT /posts/:id
Authorization: Bearer <token>
```

```json
{
  "status": "draft",
  "translations": {
    "en": {
      "title": "Updated Title",
      "content": "Updated content..."
    }
  }
}
```

### List Posts

```bash
GET /posts
```

Public users see published posts only. Authenticated users see all their posts.

**Response:**
```json
{
  "@context": "/contexts/Post",
  "@type": "hydra:Collection",
  "hydra:member": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "slug": "my-post",
      "status": "published",
      "publishedAt": "2025-02-13T10:00:00Z",
      "translations": {
        "en": {
          "title": "My Blog Post",
          "content": "Post content..."
        }
      },
      "metrics": {
        "views": 142,
        "likes": 5,
        "comments": 3
      }
    }
  ],
  "hydra:totalItems": 1
}
```

### Create Comment

```bash
POST /comments
Authorization: Bearer <token>
```

```json
{
  "commentableId": "550e8400-e29b-41d4-a716-446655440000",
  "commentable": "post",
  "content": "Great article!"
}
```

**Response:**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440000",
  "userId": "770e8400-e29b-41d4-a716-446655440000",
  "commentableId": "550e8400-e29b-41d4-a716-446655440000",
  "commentable": "post",
  "content": "Great article!",
  "status": "awaiting",
  "createdAt": "2025-02-13T10:30:00Z"
}
```

### Moderate Comment (Moderator+)

```bash
PUT /comments/:id
Authorization: Bearer <token>
```

```json
{
  "status": "published"
}
```

### Create Like

```bash
POST /likes
Authorization: Bearer <token>
```

```json
{
  "likeableId": "550e8400-e29b-41d4-a716-446655440000",
  "likeable": "post"
}
```

## Importer CLI - Bidirectional Sync

The importer CLI allows you to sync posts between your local database and external platforms like dev.to. As of version 0.5+, the importer supports **bidirectional sync**, making your local database the source of truth.

### Configuration

The importer reads database connection details from your `gorest.yaml` project configuration file. By default, it looks for `gorest.yaml` in the current directory, but you can specify a different path using the `--config` flag:

```bash
# Using gorest.yaml in current directory
./import --source devto --username yourname --user-id <uuid>

# Using gorest.yaml in a specific directory
./import --config /path/to/project --source devto --username yourname --user-id <uuid>
```

### Building the Import CLI

The import CLI can be built standalone or included in a Docker image:

**Local Build** (from your blog project):
```bash
# Build the import binary
make build-cli

# Run the import CLI
./bin/import --source devto --username yourname --user-id <uuid>
```

**Docker Build** (includes both API and import CLI):
```bash
# Build Docker image (includes /app/import binary)
docker build -t blog:latest .

# Run import CLI from Docker
docker run --rm -v $(pwd)/gorest.yaml:/app/gorest.yaml blog:latest ./import --source devto --username yourname --user-id <uuid>

# Or with environment variables
docker run --rm \
  -v $(pwd)/gorest.yaml:/app/gorest.yaml \
  -e DEVTO_API_KEY=your_key \
  blog:latest ./import --source devto --username yourname --user-id <uuid>
```

### ⚠️ Breaking Change (v0.5+)

**Default behavior has changed from `remote-wins` to `local-wins`.**

- **Old behavior (v0.4)**: Import from remote, replacing all local data
- **New behavior (v0.5+)**: Bidirectional sync - push local edits to remote and pull new remote posts

To maintain the old behavior, explicitly use `--sync-mode remote-wins`.

### Sync Modes

The importer supports three sync modes:

#### 1. `local-wins` (Default - Bidirectional)

Your local database is the source of truth. The importer will:
- ✅ Import new posts from remote (create locally)
- ✅ Push local edits to remote (update remote)
- ✅ Create remote posts for local-only posts
- ⚠️ Requires API key for pushing changes

```bash
# Default behavior (bidirectional sync)
./import --source devto --username myuser --user-id <uuid> --api-key <key>

# Explicit (same as above)
./import --source devto --username myuser --user-id <uuid> --sync-mode local-wins --api-key <key>
```

#### 2. `remote-wins` (Old Default - Import Only)

Remote platform is the source of truth. The importer will:
- ✅ Import all posts from remote
- ✅ Update existing local posts with remote content
- ❌ Does NOT push local changes back
- ℹ️ No API key required

```bash
# Import only, replace local with remote (old behavior)
./import --source devto --username myuser --user-id <uuid> --sync-mode remote-wins
```

#### 3. `import-only` (Pull New Only)

Only imports new posts, preserves existing:
- ✅ Import posts that don't exist locally
- ✅ Skip posts that already exist
- ❌ Does NOT update existing posts
- ❌ Does NOT push local changes
- ℹ️ No API key required

```bash
# Import new posts only, skip existing
./import --source devto --username myuser --user-id <uuid> --sync-mode import-only
```

### API Key Setup

For bidirectional sync (`local-wins` mode), you need a dev.to API key to push changes:

1. Visit https://dev.to/settings/extensions
2. Generate an API key
3. Provide via flag or environment variable:

```bash
# Option 1: CLI flag
./import --source devto --username myuser --user-id <uuid> --api-key YOUR_API_KEY

# Option 2: Environment variable (recommended)
export DEVTO_API_KEY=YOUR_API_KEY
./import --source devto --username myuser --user-id <uuid>
```

⚠️ **Security**: Never commit API keys to version control. Always use environment variables.

### Usage Examples

#### First-time import (bidirectional sync)
```bash
# Import all posts from dev.to and set up bidirectional sync
export DEVTO_API_KEY=your_key_here
./import --source devto --username yourname --user-id <uuid>
```

#### Edit locally, then sync back to dev.to
```bash
# 1. Edit a post locally via API
curl -X PUT http://localhost:3000/api/posts/<id> \
  -H "Authorization: Bearer <token>" \
  -d '{"translations": {"en": {"title": "Updated Title"}}}'

# 2. Sync changes back to dev.to
./import --source devto --username yourname --user-id <uuid>
# Your local edits are now pushed to dev.to
```

#### Create local post and publish to dev.to
```bash
# 1. Create post locally
curl -X POST http://localhost:3000/api/posts \
  -H "Authorization: Bearer <token>" \
  -d '{
    "slug": "new-post",
    "status": "published",
    "translations": {"en": {"title": "New Post", "content": "..."}}
  }'

# 2. Sync to create on dev.to
./import --source devto --username yourname --user-id <uuid>
# New post is created on dev.to
```

#### Reset local to match dev.to exactly
```bash
# Use remote-wins mode to replace all local with remote
./import --source devto --username yourname --user-id <uuid> --sync-mode remote-wins
```

#### Import specific article
```bash
# By URL
./import --source devto --url https://dev.to/username/article-slug-123 --user-id <uuid>

# By ID
./import --source devto --id 123456 --user-id <uuid>
```

#### Import with comments
```bash
./import --source devto --username yourname --user-id <uuid> --import-comments
```

#### Dry run (preview changes)
```bash
# See what would be synced without making changes
./import --source devto --username yourname --user-id <uuid> --dry-run
```

### CLI Flags Reference

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `--source` | Import engine to use (currently: `devto`) | No | `devto` |
| `--username` | Username to import from | Yes* | - |
| `--url` | Specific article URL to import | Yes* | - |
| `--id` | Specific article ID to import | Yes* | - |
| `--user-id` | User ID to assign posts to | Yes | - |
| `--config` | Path to gorest.yaml configuration file | No | `.` (current directory) |
| `--sync-mode` | Sync mode: `local-wins`, `remote-wins`, `import-only` | No | `local-wins` |
| `--api-key` | API key for remote platform (or use `DEVTO_API_KEY` env) | No** | - |
| `--import-comments` | Import comments along with posts | No | `false` |
| `--dry-run` | Preview import without saving | No | `false` |
| `--truncate` | Delete all posts before importing | No | `false` |
| `--list-engines` | List available import engines | No | - |

\* One of `--username`, `--url`, or `--id` must be provided
** Required for `local-wins` mode (bidirectional sync)

### Migration Guide (v0.4 → v0.5)

If you're upgrading from v0.4 and want to maintain the old import-only behavior:

**Before (v0.4):**
```bash
./import --source devto --username myuser --user-id <uuid>
```

**After (v0.5+) - Option 1: Use new bidirectional sync**
```bash
# Get API key from https://dev.to/settings/extensions
export DEVTO_API_KEY=your_key
./import --source devto --username myuser --user-id <uuid>
```

**After (v0.5+) - Option 2: Keep old behavior**
```bash
./import --source devto --username myuser --user-id <uuid> --sync-mode remote-wins
```

### Supported Platforms

- ✅ **dev.to** - Full bidirectional sync support
- 🔜 Medium (planned)
- 🔜 HashNode (planned)

## Database Schema

### Posts
- `id` (UUID, PK)
- `user_id` (UUID, FK → users)
- `slug` (TEXT, unique)
- `status` (ENUM: 'draft', 'published')
- `remote_source_id` (VARCHAR, nullable) - External platform article ID
- `remote_source` (VARCHAR, nullable) - External platform name (e.g., 'devto')
- `published_at`, `created_at`, `updated_at` (TIMESTAMP)

**Note**: Titles and content stored in `translatable` table. Remote source fields track bidirectional sync with external platforms.

### Comments
- `id` (UUID, PK)
- `user_id` (UUID, FK → users)
- `commentable_id` (UUID)
- `commentable` (TEXT: 'post', 'comment')
- `parent_id` (UUID, for nested comments)
- `content` (TEXT)
- `status` (ENUM: 'awaiting', 'published', 'draft', 'moderated')
- `ip_address`, `user_agent` (for anonymous tracking)
- `created_at`, `updated_at` (TIMESTAMP)

### Likes
- `id` (UUID, PK)
- `liker_id` (UUID, FK → users)
- `likeable_id` (UUID)
- `likeable` (TEXT: 'post', 'comment', 'user')
- `liked_id` (UUID, for user likes)
- `liked_at`, `created_at`, `updated_at` (TIMESTAMP)

### Metrics
- `post_id` (UUID, PK, FK → posts)
- `views`, `likes`, `comments` (INTEGER)
- `updated_at` (TIMESTAMP)

## Security Features

- **Authentication Required**: All write operations require JWT token
- **RBAC**: Field-level permissions via role hierarchy
- **Ownership Checks**: Users can only modify their own content
- **XSS Protection**: HTML-escaped content
- **SQL Injection Prevention**: Parameterized queries

## Performance

- **Indexed Columns**: slug, status, user_id, commentable_id
- **Efficient Queries**: Avoid N+1 with translation joins
- **Pagination**: Prevent large result sets
- **Metrics Caching**: Async view counting

## Development

### Project Structure
```
gorest-blog/
├── plugin.go              # Plugin registration
├── config.go              # Configuration
├── routes.go              # Route setup
├── models.go              # Post model with RBAC tags
├── resources.go           # CRUD handlers
├── translation_*.go       # Multi-language support
├── metrics_*.go           # Metrics tracking
├── importer_service.go    # Bidirectional sync service
├── importer/              # Import engines
│   ├── cli/               # CLI command
│   ├── engines/           # Platform-specific importers
│   │   └── devto/         # dev.to API client
│   ├── types.go           # Sync types and modes
│   └── service.go         # Import interface
├── migrations/            # Database migrations
└── types/                 # Custom types (PostStatus)
```

### Running Tests

```bash
go test ./...
```

## Troubleshooting

**Migration Error**: `relation "users" does not exist`
- Ensure `auth` plugin is enabled and loaded first

**403 Forbidden**: `insufficient permissions`
- Check user has correct role (reader/moderator/writer)
- Verify JWT token is valid

**Translation Missing**: Empty translations in response
- Ensure `translatable` plugin is enabled
- Verify translations were created with POST request

**Importer Error**: `failed to load gorest.yaml`
- Ensure `gorest.yaml` exists in the current directory or use `--config` to specify the path
- Verify the YAML syntax is correct
- Check that `database.url` is properly configured

**Importer Error**: `API key required for pushing changes`
- Set API key: `export DEVTO_API_KEY=your_key` or use `--api-key` flag
- API key only needed for `local-wins` mode (bidirectional sync)
- For import-only, use `--sync-mode remote-wins` or `--sync-mode import-only`

**Importer Error**: `user_id does not exist`
- Ensure the user ID exists in the database
- Create user via auth plugin or API first

**Sync Conflict**: Local changes overwritten
- Default mode is `local-wins` - local changes push to remote
- To pull from remote instead, use `--sync-mode remote-wins`
- Use `--dry-run` first to preview what will happen

**Importer Error**: `failed to create remote post`
- Check API key is valid (regenerate at https://dev.to/settings/extensions)
- Ensure you have write permissions on the external platform
- Check rate limits (dev.to: 10 requests per 30 seconds)

## License

MIT License - See LICENSE file for details
