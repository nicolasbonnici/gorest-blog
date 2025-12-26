# GoREST Blog Plugin - Architecture Documentation

## Overview

The GoREST Blog Plugin is a production-ready, external plugin that provides complete blog functionality for GoREST 0.4+ applications. It demonstrates best practices for plugin development, migration management, and modular architecture.

## Architecture Principles

### 1. Plugin-First Design
- Self-contained functionality
- No modifications to host application required
- Dependency injection for database and configuration
- Clean separation of concerns

### 2. Migration-Driven Schema
- Database schema defined in versioned migration files
- Multi-dialect support (PostgreSQL, MySQL, SQLite)
- Automatic dependency resolution
- Rollback capability for safe deployments

### 3. Hook-Based Extensibility
- Lifecycle hooks for data processing
- Query interception and modification
- Serialization control
- Context-aware behavior

## Component Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Host Application                      │
│  ┌───────────────────────────────────────────────────┐  │
│  │              GoREST Framework                     │  │
│  │  ┌─────────────────────────────────────────────┐  │  │
│  │  │         Plugin Registry                     │  │  │
│  │  │  ┌────────────┐  ┌───────────────────────┐  │  │  │
│  │  │  │Auth Plugin │  │    Blog Plugin        │  │  │  │
│  │  │  │            │──▶│                       │  │  │  │
│  │  │  │ • Users    │  │ • Posts               │  │  │  │
│  │  │  │ • JWT Auth │  │ • Comments            │  │  │  │
│  │  │  │            │  │ • Likes               │  │  │  │
│  │  │  │            │  │ • Importer (optional) │  │  │  │
│  │  │  └────────────┘  └───────────────────────┘  │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
│  │                                                     │  │
│  │  ┌─────────────────────────────────────────────┐  │  │
│  │  │         Migration System                    │  │  │
│  │  │  • Source Management                        │  │  │
│  │  │  • Dependency Resolution                    │  │  │
│  │  │  • Transaction Safety                       │  │  │
│  │  │  • Checksum Verification                    │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Plugin Structure

```
gorest-blog-plugin/
│
├── plugin.go                 # Plugin interface implementation
│   ├── Name()               # Returns "blog"
│   ├── Initialize()         # Configuration processing
│   ├── Handler()            # Middleware (passthrough)
│   ├── SetupEndpoints()     # Route registration
│   ├── MigrationSource()    # Migration provider
│   └── MigrationDependencies() # ["auth"]
│
├── config.go                # Configuration structure
│   └── Config               # Plugin configuration
│
├── routes.go                # Main route registration
│
├── migrations/              # Database schema (embedded)
│   ├── 20250121000001_create_posts_table.*
│   ├── 20250121000002_create_comments_table.*
│   └── 20250121000003_create_likes_table.*
│
├── models/                  # Domain models
│   ├── post.go             # Post entity
│   ├── comment.go          # Comment entity
│   └── like.go             # Like entity
│
├── resources/              # Resource handlers (CRUD)
│   ├── post_resource.go    # Post endpoints
│   ├── comment_resource.go # Comment endpoints
│   └── like_resource.go    # Like endpoints
│
├── hooks/                  # Lifecycle hooks
│   ├── post.go            # Post hooks (status, auth, etc.)
│   └── user.go            # User hooks (password hashing)
│
├── types/                 # Custom types
│   └── post_status.go    # Post status enum
│
└── importer/             # Content importer (optional)
    ├── engines/          # Import engine registry
    │   ├── engine.go    # Engine interface
    │   ├── registry.go  # Engine registration
    │   └── devto/       # Dev.to implementation
    ├── service.go       # Import orchestration
    ├── repository.go    # Data access
    └── http.go         # HTTP endpoints
```

## Data Flow

### 1. Request Processing Flow

```
HTTP Request
    │
    ▼
┌─────────────────┐
│  Fiber Router   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Plugin Handler │  (Middleware - passthrough)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Resource Handler│  (POST /posts)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ StateProcessor  │  Hook: Pre-save processing
└────────┬────────┘  • Auto-assign user_id
         │           • Set default status
         │           • Set published_at
         ▼
┌─────────────────┐
│  BaseResource   │  GoREST CRUD operations
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   BeforeQuery   │  Hook: Query modification
└────────┬────────┘  • Add status filter
         │
         ▼
┌─────────────────┐
│   Database      │  Execute query
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SerializeOne   │  Hook: Response transformation
└────────┬────────┘  • Remove sensitive data
         │
         ▼
    JSON Response
```

### 2. Migration Flow

```
Application Startup
    │
    ▼
┌──────────────────────┐
│  Plugin Registry     │
│  • Load plugins      │
│  • Call Initialize() │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Migration Manager   │
│  • Collect sources   │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Dependency Resolver  │
│  • auth → blog       │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Execute Migrations   │
│  1. auth migrations  │
│  2. blog migrations  │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Tracking Table      │
│  • Record applied    │
│  • Store checksum    │
└──────────────────────┘
```

## Key Design Decisions

### 1. Dependency Management

**Decision**: Blog plugin depends on auth plugin for users table.

**Rationale**:
- Users table required for foreign keys
- Auth provides authentication infrastructure
- Clear separation of concerns
- Reusable auth logic

**Implementation**:
```go
func (p *BlogPlugin) MigrationDependencies() []string {
    return []string{"auth"}
}
```

### 2. Embedded Migrations

**Decision**: Migrations embedded in plugin binary using `go:embed`.

**Rationale**:
- Self-contained deployment
- Version-locked schema
- No external file dependencies
- Simplifies distribution

**Implementation**:
```go
//go:embed migrations/*.sql
var migrationFiles embed.FS
```

### 3. Hook-Based Filtering

**Decision**: Use BeforeQuery hook for status filtering instead of separate endpoints.

**Rationale**:
- DRY principle (single query implementation)
- Context-aware behavior
- Transparent to API consumers
- Easier to maintain

**Trade-off**: Slightly more complex query manipulation vs. code duplication.

### 4. Polymorphic Likes

**Decision**: Single likes table with `likeable` discriminator instead of separate tables.

**Rationale**:
- Single source of truth
- Easier to query all likes
- Schema simplicity
- Extensible to new likeable types

**Trade-off**: No referential integrity on likeable_id vs. type safety.

### 5. Optional Importer

**Decision**: Importer enabled via configuration flag.

**Rationale**:
- Not all blogs need import functionality
- Reduces dependencies in minimal setups
- Separation of concerns
- Zero overhead when disabled

## Security Architecture

### Authentication Flow

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ POST /login
       │ {email, password}
       ▼
┌─────────────────┐
│  Auth Plugin    │
│  • Verify user  │
│  • Check bcrypt │
│  • Generate JWT │
└──────┬──────────┘
       │ {token, user}
       ▼
┌─────────────┐
│   Client    │ Stores token
└──────┬──────┘
       │ POST /posts
       │ Authorization: Bearer <token>
       ▼
┌─────────────────┐
│  Auth Middleware│
│  • Parse JWT    │
│  • Validate     │
│  • Set context  │
└──────┬──────────┘
       │ ctx.user_id = "uuid"
       ▼
┌─────────────────┐
│  PostHooks      │
│  • Read user_id │
│  • Assign to    │
│    post.user_id │
└─────────────────┘
```

### Security Layers

1. **Authentication**: JWT token validation
2. **Authorization**: Context-based user identification
3. **Data Sanitization**: Hook-based serialization removes passwords
4. **SQL Injection Prevention**: Parameterized queries via GoREST
5. **Password Security**: bcrypt hashing with cost factor

## Performance Considerations

### Database Optimization

1. **Indexes**:
   - Posts: slug, status, user_id, title
   - Comments: user_id, post_id, parent_id
   - Likes: liker_id, composite (likeable, likeable_id, liked_at)

2. **Foreign Keys**:
   - CASCADE deletes for data integrity
   - Indexed for join performance

3. **Pagination**:
   - Configurable limits prevent unbounded queries
   - Enforced at resource level

### Query Optimization

```sql
-- Efficient index usage
SELECT * FROM post WHERE status = 'published' ORDER BY created_at DESC LIMIT 10;
-- Uses: idx_post_status + idx_post_created_at (potential composite)

-- Efficient join
SELECT p.*, u.firstname, u.lastname
FROM post p
JOIN users u ON p.user_id = u.id;
-- Uses: idx_post_fk_user + idx_user_email
```

## Extension Points

### 1. Custom Hooks

Implement any hook interface for custom behavior:

```go
type CustomPostHook struct{}

func (h *CustomPostHook) BeforeQuery(ctx, op, query, args) {
    // Add custom filters, logging, caching, etc.
}
```

### 2. Additional Resources

Add new resources following the pattern:

```go
func RegisterTagRoutes(app *fiber.App, db database.Database) {
    baseResource := resource.NewBaseResource(db, models.Tag{}, ...)
    // Register routes
}
```

### 3. Import Engines

Register new import sources:

```go
package medium

func init() {
    engines.Register("medium", &MediumEngine{})
}
```

### 4. Custom Migrations

Add plugin-specific migrations:

```sql
-- 20250121000004_add_tags.up.postgres.sql
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL
);
```

## Testing Strategy

### Unit Tests
- Model validation
- Hook behavior
- Utility functions (slugify, etc.)

### Integration Tests
- Resource endpoints
- Authentication flow
- Hook integration with BaseResource

### Migration Tests
- Up/Down idempotency
- Multi-dialect compatibility
- Dependency ordering

### End-to-End Tests
- Complete user workflows
- Import functionality
- Permission enforcement

## Deployment Patterns

### 1. Standalone Application

```go
// Single binary with embedded plugin
import blog "github.com/nicolasbonnici/gorest-blog-plugin"
```

### 2. Multi-Plugin Architecture

```go
// Host multiple plugins
plugins := []string{"auth", "blog", "ecommerce", "analytics"}
```

### 3. Microservice per Plugin

```go
// Separate deployment units communicating via API
```

## Monitoring and Observability

### Metrics to Track

1. **Migration Health**:
   - Applied count
   - Failed migrations
   - Drift detection

2. **Resource Usage**:
   - Posts created/updated/deleted
   - Import success/failure rates
   - Query performance

3. **Authentication**:
   - Login attempts
   - Token expiration rate
   - Failed authentications

### Logging

- Migration execution (timestamps, duration)
- Hook execution (state changes)
- Import progress (fetched, created, failed)
- Authentication events

## Future Enhancements

1. **Soft Deletes**: Add deleted_at timestamp
2. **Revisions**: Track post edit history
3. **Media Management**: Image upload and storage
4. **Full-Text Search**: PostgreSQL tsvector or external search
5. **Caching Layer**: Redis integration for hot posts
6. **Rate Limiting**: Per-user limits on post creation
7. **Webhooks**: Event notifications for post actions
8. **Multi-tenancy**: Organization-scoped blogs

## Conclusion

The GoREST Blog Plugin demonstrates a comprehensive, production-ready architecture for external plugins. It balances flexibility with convention, security with usability, and modularity with simplicity. The migration system ensures safe, repeatable deployments, while hooks provide powerful extension points without coupling.
