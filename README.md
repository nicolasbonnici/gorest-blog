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

## Database Schema

### Posts
- `id` (UUID, PK)
- `user_id` (UUID, FK → users)
- `slug` (TEXT, unique)
- `status` (ENUM: 'draft', 'published')
- `published_at`, `created_at`, `updated_at` (TIMESTAMP)

**Note**: Titles and content stored in `translatable` table.

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

## License

MIT License - See LICENSE file for details
