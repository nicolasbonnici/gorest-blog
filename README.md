# GoREST Blog Plugin

[![CI](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml/badge.svg)](https://github.com/nicolasbonnici/gorest-blog/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasbonnici/gorest-blog)](https://goreportcard.com/report/github.com/nicolasbonnici/gorest-blog)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A production-ready blog plugin for GoREST 0.4+ with built-in migration support, content management, and optional content importing capabilities.

## Features

- **Complete Blog Functionality**: Posts, Comments, and Likes with hierarchical support
- **Built-in Migrations**: Automatic database schema management using GoREST 0.4 migration system
- **Multi-Database Support**: PostgreSQL, MySQL, and SQLite with dialect-specific migrations
- **Authentication Integration**: Seamless integration with GoREST auth plugin
- **Content Importer**: Optional dev.to article importer (extensible to other platforms)
- **Post Status Management**: Draft and published states with automatic timestamp handling
- **Smart Hooks**: Automatic user assignment, status filtering for unauthenticated users
- **RESTful API**: Full CRUD operations for all resources

## Installation

```bash
go get github.com/nicolasbonnici/gorest-blog-plugin
```

## Requirements

- Go 1.25.1+
- GoREST 0.4+ (feat/migrations branch)
- PostgreSQL, MySQL, or SQLite database

## Quick Start

### Basic Setup

```go
package main

import (
    "github.com/nicolasbonnici/gorest"
    "github.com/nicolasbonnici/gorest/pluginloader"

    blog "github.com/nicolasbonnici/gorest-blog-plugin"
    authplugin "github.com/nicolasbonnici/gorest/plugins/auth"
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

```yaml
database:
  url: "${DATABASE_URL}"

plugins:
  # Auth plugin is required for blog plugin
  - name: auth
    enabled: true
    config:
      jwt_secret: "${JWT_SECRET}"
      jwt_ttl: 900

  # Blog plugin
  - name: blog
    enabled: true
    config:
      pagination_limit: 10
      max_pagination_limit: 1000
      enable_importer: true  # Optional: enable dev.to importer

# Migration configuration (GoREST 0.4+)
migrations:
  enabled: true
  auto_migrate: true  # Run migrations on startup
```

## Database Schema

The plugin creates the following tables:

### Posts Table
- `id` (UUID, primary key)
- `user_id` (UUID, foreign key to users)
- `slug` (TEXT, unique)
- `status` (ENUM: 'drafted', 'published')
- `title` (TEXT)
- `content` (TEXT)
- `published_at` (TIMESTAMP)
- `created_at`, `updated_at` (TIMESTAMP)

### Comments Table
- `id` (UUID, primary key)
- `user_id` (UUID, foreign key to users)
- `post_id` (UUID, foreign key to posts)
- `parent_id` (UUID, self-reference for nested comments)
- `content` (TEXT)
- `created_at`, `updated_at` (TIMESTAMP)

### Likes Table (Polymorphic)
- `id` (UUID, primary key)
- `liker_id` (UUID, foreign key to users)
- `liked_id` (UUID, foreign key to users)
- `likeable` (TEXT: 'post' or 'comment')
- `likeable_id` (UUID)
- `liked_at` (TIMESTAMP)

## Migration System

The blog plugin uses GoREST 0.4's migration system with the following features:

### Automatic Migration on Startup

Migrations run automatically when `migrations.auto_migrate: true` is set. The plugin:
1. Depends on the `auth` plugin (ensures users table exists first)
2. Creates `post_status` enum type
3. Creates posts, comments, and likes tables
4. Sets up all necessary indexes

### Manual Migration Control

```bash
# Run pending migrations
gorest migrate up

# Run migrations for specific plugin
gorest migrate up --source blog

# Rollback last migration
gorest migrate down

# Check migration status
gorest migrate status
```

### Migration Files

Located in `migrations/` directory:
- `20250121000001_create_posts_table.{up,down}.postgres.sql`
- `20250121000002_create_comments_table.{up,down}.postgres.sql`
- `20250121000003_create_likes_table.{up,down}.postgres.sql`

## API Endpoints

### Posts

- `GET /posts` - List all published posts (public) or all posts (authenticated)
- `GET /posts/:id` - Get a specific post
- `POST /posts` - Create a new post (authenticated)
- `PUT /posts/:id` - Update a post (authenticated)
- `DELETE /posts/:id` - Delete a post (authenticated)

### Comments

- `GET /comments` - List all comments
- `GET /comments/:id` - Get a specific comment
- `POST /comments` - Create a new comment (authenticated)
- `PUT /comments/:id` - Update a comment (authenticated)
- `DELETE /comments/:id` - Delete a comment (authenticated)

### Likes

- `GET /likes` - List all likes
- `GET /likes/:id` - Get a specific like
- `POST /likes` - Like a post or comment (authenticated)
- `DELETE /likes/:id` - Unlike (authenticated)

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

```go
// Handled automatically by PostHooks
POST /posts
{
  "title": "My Post",
  "content": "Post content"
}
// user_id is set from JWT token automatically
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

When a post status changes to 'published', `published_at` is set automatically:

```json
{
  "status": "published"
}
// published_at set to current timestamp automatically
```

### Password Hashing

User passwords are automatically hashed using bcrypt (via UserHooks).

## Advanced Configuration

### Custom Pagination

```go
config := map[string]interface{}{
    "pagination_limit": 20,
    "max_pagination_limit": 500,
}
```

### Plugin Dependencies

The blog plugin declares a dependency on the auth plugin:

```go
func (p *BlogPlugin) MigrationDependencies() []string {
    return []string{"auth"}
}
```

This ensures:
1. Auth plugin migrations run first
2. Users table exists before creating posts
3. Foreign key constraints work correctly

## Development

### Project Structure

```
gorest-blog-plugin/
├── plugin.go              # Main plugin implementation
├── config.go              # Configuration structure
├── routes.go              # Route registration
├── migrations/            # Database migrations
│   ├── 20250121000001_create_posts_table.{up,down}.postgres.sql
│   ├── 20250121000002_create_comments_table.{up,down}.postgres.sql
│   └── 20250121000003_create_likes_table.{up,down}.postgres.sql
├── models/                # Data models
│   ├── post.go
│   ├── comment.go
│   └── like.go
├── resources/             # Resource handlers
│   ├── post_resource.go
│   ├── comment_resource.go
│   └── like_resource.go
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

### Adding Custom Hooks

```go
type PostHooks struct{}

func (h *PostHooks) StateProcessor(ctx context.Context, operation hooks.Operation, id any, post *models.Post) error {
    // Custom logic before save
    return nil
}

func (h *PostHooks) BeforeQuery(ctx context.Context, operation hooks.Operation, query string, args []any) (string, []any, error) {
    // Modify queries dynamically
    return query, args, nil
}
```

## Production Considerations

### Security

- All write operations require authentication
- Passwords are bcrypt-hashed
- SQL injection prevention via parameterized queries
- JWT token validation

### Performance

- Indexed columns: slug, status, user_id, post_id
- Pagination support prevents large result sets
- Efficient composite indexes for likes

### Scalability

- Stateless design (compatible with horizontal scaling)
- Database connection pooling (via GoREST)
- Migration checksums prevent drift

## Extending the Plugin

### Add New Import Engines

```go
package myengine

import "github.com/nicolasbonnici/gorest-blog-plugin/importer/engines"

type MyEngine struct{}

func init() {
    engines.Register("myengine", &MyEngine{})
}

func (e *MyEngine) FetchByUsername(ctx context.Context, username string) ([]Post, error) {
    // Implementation
}
```

### Custom Post Types

Extend the `post_status` enum:

```sql
-- Create migration: 20250121000004_add_archived_status.up.postgres.sql
ALTER TYPE post_status ADD VALUE 'archived';
```

## Testing

```bash
# Run tests
go test ./...

# Test with coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./...
```

## Troubleshooting

### Migration Errors

**Problem**: `migration failed: relation "users" does not exist`

**Solution**: Ensure auth plugin is enabled and loaded before blog plugin.

**Problem**: `migration checksum mismatch`

**Solution**: Migration files were modified after being applied. Use `gorest migrate force` to mark as applied (destructive).

### Import Errors

**Problem**: `user_id does not exist`

**Solution**: Create a user first via `/login` endpoint or insert directly into database.

## License

MIT License - See LICENSE file for details

## Contributing

Contributions welcome! Please submit issues and pull requests on GitHub.

## Changelog

### v1.0.0 (2025-01-21)
- Initial release
- PostgreSQL, MySQL, SQLite support
- GoREST 0.4 migration system integration
- Dev.to importer
- Complete CRUD operations
- Authentication integration
- Smart hooks and filters
