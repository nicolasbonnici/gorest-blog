# GoREST Blog Plugin

[![CI](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml/badge.svg)](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasbonnici/gorest-blog)](https://goreportcard.com/report/github.com/nicolasbonnici/gorest-blog)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

An autonomous, production-ready blog plugin for GoREST 0.4+ with built-in migration support, post management, and optional content importing capabilities.

## Features

- **Post Management**: Complete CRUD operations for blog posts
- **Built-in Migrations**: Automatic database schema management using Go migrations (no SQL files)
- **Multi-Database Support**: PostgreSQL, MySQL, and SQLite with dialect-specific migrations
- **Authentication Integration**: Seamless integration with GoREST auth plugin
- **Content Importer**: Optional dev.to article importer (extensible to other platforms)
- **Post Status Management**: Draft and published states with automatic timestamp handling
- **Smart Hooks**: Automatic user assignment, status filtering for unauthenticated users
- **RESTful API**: Full CRUD operations for posts
- **Autonomous**: No dependencies on other blog-related plugins

## Architecture

This plugin follows a **microservices-inspired** approach where each concern is separated:

- **gorest-blog**: Manages posts only
- **gorest-commentable**: Adds polymorphic commenting (optional, separate plugin)
- **gorest-likeable**: Adds polymorphic likes (optional, separate plugin)

**No plugin depends on another** - your application decides which plugins to enable.

## Installation

```bash
go get github.com/nicolasbonnici/gorest-blog
```

## Requirements

- Go 1.25.1+
- GoREST 0.4+
- PostgreSQL, MySQL, or SQLite database

## Quick Start

### Basic Setup (Posts Only)

```go
package main

import (
    "github.com/nicolasbonnici/gorest"
    "github.com/nicolasbonnici/gorest/pluginloader"

    blog "github.com/nicolasbonnici/gorest-blog"
    authplugin "github.com/nicolasbonnici/gorest-auth"
)

func init() {
    // Register auth plugin (required dependency)
    pluginloader.RegisterPluginFactory("auth", authplugin.NewPlugin)

    // Register blog plugin
    pluginloader.RegisterPluginFactory("blog", blog.NewPlugin)
}

func main() {
    cfg := gorest.Config{
        ConfigPath: ".",
    }

    gorest.Start(cfg)
}
```

### Configuration (gorest.yaml)

#### Minimal Configuration (Posts Only)

```yaml
database:
  url: "${DATABASE_URL}"

plugins:
  - name: auth
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"

  - name: blog
    enabled: true
    config:
      pagination_limit: 10
      max_pagination_limit: 100

migrations:
  enabled: true
  auto_migrate: true
```

#### Full Stack (Posts + Comments + Likes)

```yaml
database:
  url: "${DATABASE_URL}"

plugins:
  - name: auth
    enabled: true

  - name: blog
    enabled: true
    config:
      pagination_limit: 10
      enable_importer: true

  - name: commentable
    enabled: true
    config:
      allowed_types: ["post"]
      max_content_length: 10000

  - name: likeable
    enabled: true
    config:
      allowed_types: ["post", "comment"]

migrations:
  enabled: true
  auto_migrate: true
```

## Database Schema

The plugin creates the **posts table only**:

### Posts Table
- `id` (UUID, primary key)
- `user_id` (UUID, foreign key to users)
- `slug` (TEXT)
- `status` (ENUM: 'drafted', 'published')
- `title` (TEXT)
- `content` (TEXT)
- `published_at` (TIMESTAMP)
- `created_at`, `updated_at` (TIMESTAMP)

### Indexes
- `idx_post_title` on `title`
- `idx_post_status` on `status`
- `idx_post_fk_user` on `user_id`
- `idx_post_slug` on `slug`

> **Note**: Comments and likes are managed by separate plugins (`gorest-commentable` and `gorest-likeable`)

## Migration System

The blog plugin uses **Go migrations** (not SQL files) via GoREST 0.4's migration system.

### Automatic Migration on Startup

Migrations run automatically when `migrations.auto_migrate: true` is set. The plugin:
1. Depends on the `auth` plugin (ensures users table exists first)
2. Creates `post_status` enum type (Postgres) or CHECK constraint (SQLite/MySQL)
3. Creates posts table
4. Sets up all necessary indexes

### Migration Dependencies

```go
func (p *BlogPlugin) MigrationDependencies() []string {
    return []string{"auth"}  // Only depends on auth, NOT on commentable or likeable
}
```

## API Endpoints

### Posts

- `GET /posts` - List all published posts (public) or all posts (authenticated)
- `GET /posts/:id` - Get a specific post
- `POST /posts` - Create a new post (authenticated)
- `PUT /posts/:id` - Update a post (authenticated)
- `DELETE /posts/:id` - Delete a post (authenticated)

#### Query Examples

```bash
# Get all published posts
GET /posts?status=published&orderBy=created_at:desc

# Get user's posts
GET /posts?userId={uuid}&orderBy=created_at:desc

# Pagination
GET /posts?limit=20&page=2
```

### Content Importer (Optional)

- `GET /api/import/engines` - List available import engines
- `POST /api/import/:engine` - Import content from external source

#### Import Request Example

```json
{
  "username": "devto_username",
  "user_id": "uuid-of-user",
  "update_existing": false,
  "dry_run": false
}
```

## Smart Features

### Automatic User Assignment

Posts automatically assign the authenticated user's ID on creation:

```bash
POST /posts
Authorization: Bearer <token>
{
  "title": "My Post",
  "content": "Post content"
}
# user_id is set from JWT token automatically
```

### Status Filtering

Unauthenticated users only see published posts:

```bash
# Public user - only sees published posts
curl http://localhost:8000/posts

# Authenticated user - sees all posts
curl -H "Authorization: Bearer <token>" http://localhost:8000/posts
```

### Auto-Published Timestamp

When a post status changes to 'published', `published_at` is set automatically.

## Plugin Composition

### Adding Comments (gorest-commentable)

```go
import commentable "github.com/nicolasbonnici/gorest-commentable"

func init() {
    pluginloader.RegisterPluginFactory("commentable", commentable.NewPlugin)
}
```

```yaml
plugins:
  - name: commentable
    enabled: true
    config:
      allowed_types: ["post"]  # Allow comments on posts
      max_content_length: 10000
```

### Adding Likes (gorest-likeable)

```go
import likeable "github.com/nicolasbonnici/gorest-likeable"

func init() {
    pluginloader.RegisterPluginFactory("likeable", likeable.NewPlugin)
}
```

```yaml
plugins:
  - name: likeable
    enabled: true
    config:
      allowed_types: ["post", "comment"]  # Like posts and comments
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `pagination_limit` | `int` | `10` | Default pagination limit |
| `max_pagination_limit` | `int` | `100` | Maximum allowed pagination limit |
| `enable_importer` | `bool` | `false` | Enable content import endpoints |

## Development

### Project Structure

```
gorest-blog/
├── plugin.go              # Main plugin implementation
├── config.go              # Configuration structure
├── routes.go              # Route registration
├── migrations/            # Go migrations (no SQL files)
│   └── migrations.go
├── models/                # Data models
│   └── post.go
├── resources/             # Resource handlers
│   └── post_resource.go
├── hooks/                 # Lifecycle hooks
│   ├── post.go
│   └── user.go
├── types/                 # Custom types
│   └── post_status.go
└── importer/             # Content importer (optional)
    ├── engines/
    │   └── devto/
    └── ...
```

### Run Tests

```bash
make test
```

### Run Linter

```bash
make lint
```

### Build

```bash
make build
```

## Production Considerations

### Security

- All write operations require authentication
- SQL injection prevention via parameterized queries
- JWT token validation
- XSS protection (if using with commentable plugin)

### Performance

- Indexed columns: slug, status, user_id
- Pagination support prevents large result sets
- Efficient database queries

### Scalability

- Stateless design (compatible with horizontal scaling)
- Database connection pooling (via GoREST)
- No cross-plugin dependencies

## Extending the Plugin

### Add New Import Engines

```go
package myengine

import "github.com/nicolasbonnici/gorest-blog/importer/engines"

type MyEngine struct{}

func init() {
    engines.Register("myengine", &MyEngine{})
}

func (e *MyEngine) Import(ctx context.Context, config ImportConfig) (*ImportResult, error) {
    // Implementation
}
```

## Troubleshooting

### Migration Errors

**Problem**: `migration failed: relation "users" does not exist`

**Solution**: Ensure auth plugin is enabled and loaded before blog plugin.

### Import Errors

**Problem**: `user_id does not exist`

**Solution**: Create a user first via `/login` endpoint or insert directly into database.

## License

MIT License - See LICENSE file for details

## Contributing

Contributions welcome! Please ensure:
- All tests pass
- Code is linted
- New features have test coverage
- Documentation is updated

## Part of GoREST Ecosystem

- [GoREST](https://github.com/nicolasbonnici/gorest) - Core framework
- [GoREST Auth](https://github.com/nicolasbonnici/gorest-auth) - Authentication plugin
- [GoREST Commentable](https://github.com/nicolasbonnici/gorest-commentable) - Polymorphic commenting (optional)
- [GoREST Likeable](https://github.com/nicolasbonnici/gorest-likeable) - Polymorphic likes (optional)

## Changelog

### v2.0.0 (2026-01-02)
- **BREAKING**: Removed comments and likes - moved to separate plugins
- Migration to Go migrations (no more SQL files)
- Autonomous plugin design - no dependencies on other blog plugins
- Improved plugin composition architecture

### v1.0.0 (2025-01-21)
- Initial release with posts, comments, and likes
