# GoREST Blog Plugin

[![CI](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml/badge.svg)](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nicolasbonnici/gorest-blog.svg)](https://pkg.go.dev/github.com/nicolasbonnici/gorest-blog)
[![Go Version](https://img.shields.io/github/go-mod/go-version/nicolasbonnici/gorest-blog)](https://github.com/nicolasbonnici/gorest-blog/blob/HEAD/go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A production-ready blog plugin for GoREST with multi-language support, RBAC, and metrics.

## Features

- **Multi-language posts** — titles and content per locale via the `translatable` plugin
- **RBAC** — reader / moderator / writer roles with field-level permissions
- **Metrics** — views, likes, and comments
- **Multi-database** — PostgreSQL, MySQL, SQLite
- **Bidirectional sync** — import from and push to external platforms (dev.to)
- **Built-in migrations** — automatic schema management

## Requirements

- Go 1.25.1+
- GoREST 0.4+
- PostgreSQL, MySQL, or SQLite

## Setup

```bash
make install   # deps, dev tools (golangci-lint), git hooks
make audit     # run quality checks (CI parity)
make test      # tests with race detector
```

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

## API

All write operations require `Authorization: Bearer <token>`.

### Posts

**`POST /posts`** — create
```json
{
  "slug": "my-post",
  "status": "published",
  "translations": {
    "en": {"title": "My Blog Post", "content": "Content in English..."},
    "fr": {"title": "Mon Article", "content": "Contenu en français..."}
  }
}
```

**`PUT /posts/:id`** — update (metadata, translations, or both)
```json
{
  "status": "draft",
  "translations": {"en": {"title": "Updated", "content": "..."}}
}
```

**`GET /posts`** — list (public sees published only; authenticated users see all their own).
Supports `?category=<slug>` and `?tag=<slug>` filters when the `taxonomy` plugin is loaded.

**Response shape:**
```json
{
  "id": "550e8400-...",
  "slug": "my-post",
  "status": "published",
  "publishedAt": "2025-02-13T10:00:00Z",
  "translations": { "en": {"title": "...", "content": "..."} },
  "metrics": { "views": 142, "likes": 5, "comments": 3 }
}
```

### Comments

**`POST /comments`**
```json
{
  "commentableId": "550e8400-...",
  "commentable": "post",
  "content": "Great article!"
}
```

**`PUT /comments/:id`** — moderate (moderator+)
```json
{ "status": "published" }
```

### Likes

**`POST /likes`**
```json
{ "likeableId": "550e8400-...", "likeable": "post" }
```

## Importer CLI — Bidirectional Sync

Syncs posts between your local database and external platforms. Your local database is the source of truth.

### Build

```bash
# From the blog project
make build-cli
./bin/import --source devto --username yourname --user-id <uuid>

# Docker (image bundles /app/import)
docker build -t blog:latest .
docker run --rm -v $(pwd)/gorest.yaml:/app/gorest.yaml \
  -e DEVTO_API_KEY=$DEVTO_API_KEY \
  blog:latest ./import --source devto --username yourname --user-id <uuid>
```

The importer reads DB connection from `gorest.yaml` in the current directory; override with `--config /path/to/project`.

### Sync Modes

| Mode | Direction | API key | Behavior |
|------|-----------|---------|----------|
| `local-wins` *(default)* | Bidirectional | Required | Pulls new remote posts, pushes local edits, creates remote posts for local-only |
| `remote-wins` | Pull only | No | Imports all remote posts, overwrites matching local ones |
| `import-only` | Pull new only | No | Imports posts that don't exist locally; skips existing |

```bash
./import --source devto --username myuser --user-id <uuid>                          # bidirectional
./import --source devto --username myuser --user-id <uuid> --sync-mode remote-wins  # pull-only
./import --source devto --username myuser --user-id <uuid> --sync-mode import-only  # pull new
```

### API Key

Bidirectional sync (`local-wins`) needs a dev.to API key from https://dev.to/settings/extensions:

```bash
export DEVTO_API_KEY=your_key   # recommended
# or: --api-key YOUR_API_KEY
```

> **Security**: never commit API keys. Use environment variables.

### Common Usage

```bash
# First-time bidirectional import
export DEVTO_API_KEY=...
./import --source devto --username yourname --user-id <uuid>

# Sync local edits back to dev.to (after editing via API)
./import --source devto --username yourname --user-id <uuid>

# Reset local to match dev.to
./import --source devto --username yourname --user-id <uuid> --sync-mode remote-wins

# Import a specific article
./import --source devto --url https://dev.to/username/article-slug-123 --user-id <uuid>
./import --source devto --id 123456 --user-id <uuid>

# Preview without writing
./import --source devto --username yourname --user-id <uuid> --dry-run

# Include comments
./import --source devto --username yourname --user-id <uuid> --import-comments
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--source` | Import engine (`devto`) | `devto` |
| `--username` / `--url` / `--id` | What to import (one required) | – |
| `--user-id` | User ID to assign posts to (required) | – |
| `--config` | Path to `gorest.yaml` | `.` |
| `--sync-mode` | `local-wins` / `remote-wins` / `import-only` | `local-wins` |
| `--api-key` | API key (or `DEVTO_API_KEY` env) | – |
| `--import-comments` | Import comments too | `false` |
| `--dry-run` | Preview without saving | `false` |
| `--truncate` | Delete all posts before importing | `false` |
| `--list-engines` | List available engines | – |

### Supported Platforms

- dev.to — full bidirectional sync
- Medium, HashNode — planned

## Database Schema

**Posts** — `id`, `user_id`, `slug` (unique), `status` (`draft`/`published`), `remote_source_id`, `remote_source`, `published_at`, `created_at`, `updated_at`.
Titles and content live in the `translatable` table.

**Comments** — `id`, `user_id`, `commentable_id`, `commentable` (`post`/`comment`), `parent_id`, `content`, `status` (`awaiting`/`published`/`draft`/`moderated`), `ip_address`, `user_agent`, timestamps.

**Likes** — `id`, `liker_id`, `likeable_id`, `likeable` (`post`/`comment`/`user`), `liked_id`, `liked_at`, timestamps.

**Metrics** — `post_id` (PK/FK), `views`, `likes`, `comments`, `updated_at`.

## Security & Performance

- JWT-required writes, RBAC field-level permissions, ownership checks
- HTML-escaped titles (content is markdown — render with a sanitizing processor)
- Parameterized queries via the `gorest/query` builder
- Indexed: `slug`, `status`, `user_id`, `commentable_id`
- Async view counting, paginated list endpoints

## Project Structure

```
gorest-blog/
├── plugin.go              # Plugin registration
├── config.go              # Configuration
├── routes.go              # Route setup
├── resources.go           # CRUD handlers
├── models/                # Post model with RBAC tags
├── hooks/                 # Pre/post hooks (translations, taxonomy, AI)
├── services/              # Translation, metrics services
├── importer/              # Import engines (cli, engines/devto)
├── importer_service.go    # Bidirectional sync service
├── migrations/            # Database migrations
└── types/                 # Custom types (PostStatus)
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `relation "users" does not exist` | Enable and load `auth` plugin first |
| `403 insufficient permissions` | Check role (reader/moderator/writer) and JWT validity |
| Empty translations in response | Ensure `translatable` plugin is enabled; verify translations were sent on POST |
| `failed to load gorest.yaml` | File missing or invalid; check `database.url`, use `--config` if needed |
| `API key required for pushing changes` | Set `DEVTO_API_KEY` or use `--sync-mode remote-wins`/`import-only` |
| `user_id does not exist` | Create the user via auth plugin or API first |
| Local changes overwritten | Default is `local-wins`; for pull use `--sync-mode remote-wins`. Use `--dry-run` to preview |
| `failed to create remote post` | Check API key, write permissions, and rate limits (dev.to: 10 req / 30s) |

## License

MIT — see [LICENSE](LICENSE).
