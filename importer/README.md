# Importer Plugin

A flexible blog post importer plugin for the GoREST blog application with support for multiple blog platforms.

## Architecture

The importer plugin uses an extensible **engine-based architecture** that allows you to import blog posts from various platforms (Dev.to, Medium, Hashnode, etc.) with a consistent interface.

```
plugins/importer/
├── cli/                 # CLI implementation (owned by plugin)
│   └── cli.go          # CLI logic with Run() function
├── engines/             # Platform-specific implementations
│   ├── registry.go     # Engine auto-registration system
│   ├── devto/          # Dev.to engine
│   └── [future]/       # Medium, Hashnode, etc.
├── service.go          # Core import orchestration
├── repository.go       # Database operations
├── http.go             # REST API handlers
├── plugin.go           # Plugin implementation
└── types.go            # Shared types

cmd/import/
└── main.go             # Thin wrapper (6 lines, calls cli.Run())
```

### Plugin-Owned CLI

All CLI logic lives in `plugins/importer/cli/cli.go`. The `cmd/import/main.go` file is just a thin wrapper that calls the plugin's `Run()` function. This means:

- **Single source of truth**: All import logic (CLI and HTTP) is in the plugin
- **Extensible**: Add new engines without modifying CLI code
- **Maintainable**: Update CLI behavior by editing the plugin
- **Auto-installed**: `make build-cli` automatically creates the wrapper if missing

## Features

- **Multiple Engines**: Support for Dev.to (with more platforms coming)
- **Dual Interface**: Both CLI and HTTP REST API
- **Auto-Registration**: Engines register themselves via `init()` functions
- **Progress Tracking**: Real-time progress bars in CLI
- **Duplicate Detection**: Update existing posts or skip duplicates
- **Dry-Run Mode**: Preview imports without saving
- **Extensible**: Add new engines by implementing the `Engine` interface

## Usage

### CLI Tool

Build the CLI tool:

```bash
make build-cli
```

#### List Available Engines

```bash
./bin/import --list-engines
```

Output:
```
Available import engines:
  - devto
```

#### Import from Dev.to

**Import all articles from a user:**
```bash
./bin/import \
  --source devto \
  --username nicolasbonnici \
  --user-id <your-uuid>
```

**Import specific article by URL:**
```bash
./bin/import \
  --source devto \
  --url https://dev.to/username/article-slug-123 \
  --user-id <your-uuid>
```

**Import specific article by ID:**
```bash
./bin/import \
  --source devto \
  --id 123456 \
  --user-id <your-uuid>
```

**Update existing posts with matching titles:**
```bash
./bin/import \
  --source devto \
  --username nicolasbonnici \
  --user-id <your-uuid> \
  --update
```

**Dry-run (preview without saving):**
```bash
./bin/import \
  --source devto \
  --username nicolasbonnici \
  --user-id <your-uuid> \
  --dry-run
```

#### CLI Flags

| Flag | Description | Required |
|------|-------------|----------|
| `--source` | Engine to use (default: `devto`) | No |
| `--username` | Username to import articles from | * |
| `--url` | Specific article URL to import | * |
| `--id` | Specific article ID to import | * |
| `--user-id` | User ID to assign imported posts to | Yes |
| `--update` | Update existing posts with matching titles | No |
| `--dry-run` | Preview import without saving | No |
| `--list-engines` | List available engines | No |

\* At least one of `--username`, `--url`, or `--id` must be provided.

### HTTP REST API

Start the API server:

```bash
make run
```

#### List Available Engines

```bash
curl http://localhost:3000/api/import/engines
```

Response:
```json
{
  "engines": ["devto"]
}
```

#### Import from Dev.to

**Import by username:**
```bash
curl -X POST http://localhost:3000/api/import/devto \
  -H "Content-Type: application/json" \
  -d '{
    "username": "nicolasbonnici",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "update_existing": true
  }'
```

**Import by URL:**
```bash
curl -X POST http://localhost:3000/api/import/devto \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://dev.to/username/article-slug-123",
    "user_id": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Import by ID:**
```bash
curl -X POST http://localhost:3000/api/import/devto \
  -H "Content-Type: application/json" \
  -d '{
    "id": "123456",
    "user_id": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Dry-run:**
```bash
curl -X POST http://localhost:3000/api/import/devto \
  -H "Content-Type: application/json" \
  -d '{
    "username": "nicolasbonnici",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "dry_run": true
  }'
```

#### Response Format

Success response:
```json
{
  "success": true,
  "message": "Import completed: 10 fetched, 8 created, 2 updated, 0 skipped, 0 failed",
  "total_fetched": 10,
  "created": 8,
  "updated": 2,
  "skipped": 0,
  "failed": 0,
  "errors": []
}
```

Error response:
```json
{
  "success": false,
  "message": "Import failed: ...",
  "total_fetched": 0,
  "created": 0,
  "updated": 0,
  "skipped": 0,
  "failed": 1,
  "errors": ["error message here"]
}
```

## How It Works

### Import Flow

1. **Fetch**: Retrieve posts from the blog platform via its API
2. **Transform**: Convert platform-specific format to normalized `Post` struct
3. **Deduplicate**: Check for existing posts by title
4. **Persist**:
   - If post exists + `update_existing=true`: **Update**
   - If post exists + `update_existing=false`: **Skip**
   - If post doesn't exist: **Create**
5. **Report**: Return statistics (created, updated, skipped, failed)

### Engine Auto-Registration

Engines register themselves automatically when imported:

```go
// In engines/devto/engine.go
func init() {
    engines.Register(NewEngine())
}

// In plugin.go or cmd/import/main.go
import (
    _ "github.com/nicolasbonnici/blog/plugins/importer/engines/devto"
)
```

The blank import (`_`) triggers the `init()` function, which registers the engine in the global registry.

## Adding New Engines

Adding support for a new blog platform is straightforward:

### Step 1: Create Engine Directory

```bash
mkdir -p plugins/importer/engines/medium
```

### Step 2: Implement Engine Interface

Create `plugins/importer/engines/medium/engine.go`:

```go
package medium

import (
    "context"
    "github.com/nicolasbonnici/blog/plugins/importer"
    "github.com/nicolasbonnici/blog/plugins/importer/engines"
)

// Auto-register on package import
func init() {
    engines.Register(NewEngine())
}

type Engine struct {
    client *Client  // Your HTTP client
}

func NewEngine() *Engine {
    return &Engine{
        client: NewClient(),
    }
}

func (e *Engine) Name() string {
    return "medium"
}

func (e *Engine) FetchByUsername(ctx context.Context, username string) ([]importer.Post, error) {
    // Fetch posts from Medium API
    // Convert to []importer.Post
}

func (e *Engine) FetchByID(ctx context.Context, id string) (*importer.Post, error) {
    // Fetch single post from Medium API
    // Convert to *importer.Post
}

func (e *Engine) FetchByURL(ctx context.Context, url string) (*importer.Post, error) {
    // Fetch post from Medium API by URL
    // Convert to *importer.Post
}
```

### Step 3: Import the Engine

Add to `plugins/importer/plugin.go` and `cmd/import/main.go`:

```go
import (
    _ "github.com/nicolasbonnici/blog/plugins/importer/engines/devto"
    _ "github.com/nicolasbonnici/blog/plugins/importer/engines/medium"  // Add this
)
```

### Step 4: Use the Engine

The new engine is automatically available:

**CLI:**
```bash
./bin/import --source medium --username yourname --user-id <uuid>
```

**HTTP:**
```bash
curl -X POST http://localhost:3000/api/import/medium \
  -H "Content-Type: application/json" \
  -d '{"username": "yourname", "user_id": "uuid"}'
```

That's it! No changes needed to core logic.

## Engine Interface

To create a new engine, implement this interface:

```go
type Engine interface {
    // Name returns the engine identifier (e.g., "devto", "medium")
    Name() string

    // FetchByUsername fetches all posts for a given username
    FetchByUsername(ctx context.Context, username string) ([]Post, error)

    // FetchByID fetches a single post by platform-specific ID
    FetchByID(ctx context.Context, id string) (*Post, error)

    // FetchByURL fetches a single post by its URL
    FetchByURL(ctx context.Context, url string) (*Post, error)
}
```

### Post Struct

All engines must convert their platform-specific data to this normalized struct:

```go
type Post struct {
    ID          string    // Platform-specific ID
    Title       string    // Post title
    Content     string    // Post body (markdown or HTML)
    PublishedAt string    // RFC3339 timestamp
    UpdatedAt   string    // RFC3339 timestamp (optional)
    URL         string    // Original post URL
    SourceID    string    // Platform identifier (e.g., "devto:123456")
    Tags        []string  // Post tags (optional)
    CoverImage  string    // Cover image URL (optional)
}
```

## Dev.to Engine

The Dev.to engine uses the public Dev.to API (no API key required for public articles).

### API Endpoints

- `GET https://dev.to/api/articles?username={username}` - Fetch user's articles
- `GET https://dev.to/api/articles/{id}` - Fetch specific article

### Field Mapping

| Dev.to Field | Post Field | Post Model Field |
|--------------|---------------|------------------|
| `id` | `ID` | - |
| `title` | `Title` | `title` |
| `body_markdown` | `Content` | `content` |
| `published_at` | `PublishedAt` | `created_at` |
| `edited_at` | `UpdatedAt` | `updated_at` |
| `url` | `URL` | - |
| - | - | `user_id` (from flag) |

## Configuration

### Environment Variables

Required:
- `DATABASE_URL` - PostgreSQL connection string (CLI only)

Optional:
- Engine-specific API keys can be added as needed

### Plugin Configuration

In `gorest.yaml`:

```yaml
plugins:
  - name: importer
    enabled: true
```

## Error Handling

The importer provides detailed error reporting:

- **CLI**: Errors are printed to stderr with context
- **HTTP**: Errors are returned in the `errors` array with HTTP status codes

Common error scenarios:
- Invalid user ID
- Network errors (API unreachable)
- Post not found
- Database errors
- Duplicate detection failures

## Testing

Test the importer with a dry-run first:

```bash
./bin/import \
  --source devto \
  --username someuser \
  --user-id <uuid> \
  --dry-run
```

This will:
1. Fetch posts from Dev.to
2. Check for duplicates
3. Show what would be created/updated
4. **NOT** save anything to the database

## Troubleshooting

### "unknown source: xyz"
The engine is not registered. Make sure to import it with a blank import (`_`).

### "user_id is required"
You must provide a valid user UUID via `--user-id` flag or `user_id` JSON field.

### "one of username, url, or id must be provided"
Specify at least one import method: `--username`, `--url`, or `--id`.

### Database connection errors (CLI)
Set the `DATABASE_URL` environment variable:
```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/blog?sslmode=disable"
```

### API timeout (HTTP)
Long imports (100+ articles) may timeout. Consider:
- Implementing async import with background jobs
- Paginating large imports
- Increasing timeout in `http.go`

## Performance

- **CLI**: Processes articles sequentially with progress updates
- **HTTP**: 5-minute timeout per import request
- **Database**: Uses existing CRUD layer with connection pooling

For large imports (500+ articles), consider:
- Batch processing
- Background job queue
- Pagination

## License

Same as the parent project.
