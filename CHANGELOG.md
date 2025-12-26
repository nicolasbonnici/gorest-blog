# Changelog

All notable changes to the GoREST Blog Plugin will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-01-21

### Added

#### Core Features
- Complete blog plugin implementation for GoREST 0.4
- Full CRUD operations for Posts, Comments, and Likes
- GoREST 0.4 migration system integration
- Multi-database support (PostgreSQL, MySQL, SQLite)
- Dependency management (depends on auth plugin)
- Embedded migrations using go:embed

#### Models
- Post model with status enum (drafted/published)
- Comment model with hierarchical support (parent_id)
- Like model with polymorphic support (posts/comments)

#### Migrations
- `20250121000001_create_posts_table` - Posts table with indexes
- `20250121000002_create_comments_table` - Comments with hierarchy
- `20250121000003_create_likes_table` - Polymorphic likes

#### Hooks
- PostHooks with StateProcessor (auto user assignment, status handling)
- PostHooks with BeforeQuery (status filtering for unauthenticated users)
- PostHooks with SerializeOne/Many (response transformation)
- UserHooks with StateProcessor (password hashing)
- UserHooks with SerializeOne/Many (password hiding)

#### Resources
- Post resource with full CRUD
- Comment resource with full CRUD
- Like resource with full CRUD
- Pagination support on all resources
- Authentication integration

#### Configuration
- Configurable pagination limits
- Optional importer enable/disable
- Database injection
- Plugin lifecycle management

#### Content Importer
- Dev.to engine implementation
- Extensible engine registry
- HTTP API endpoints (`/api/import/:engine`)
- CLI tool for command-line imports
- Progress reporting system
- Dry-run support
- Update existing posts option
- Import by username, URL, or ID

#### Documentation
- README.md with quick start and API reference
- ARCHITECTURE.md with design decisions and diagrams
- MIGRATION_GUIDE.md with complete migration tutorial
- IMPLEMENTATION_SUMMARY.md with project overview
- Example application in `examples/basic/`
- Inline code documentation

#### Developer Experience
- Complete example application
- Environment variable templates
- Production-ready configuration examples
- Troubleshooting guides
- Best practices documentation

#### Security
- JWT authentication integration
- Bcrypt password hashing
- SQL injection prevention (parameterized queries)
- Unauthenticated access filtering
- Context-aware user assignment

#### Performance
- Database indexes on all foreign keys
- Composite indexes for efficient queries
- Pagination enforcement
- Optimized hook execution

### Dependencies
- GoREST v0.4.0 (feat/migrations branch)
- Fiber v2.52.10
- golang.org/x/crypto v0.46.0
- progressbar v3.14.1 (for importer)

### Breaking Changes
None (initial release)

### Known Issues
None

### Migration Path
This is the initial release. To use:
1. Install: `go get github.com/nicolasbonnici/gorest-blog-plugin`
2. Register plugin in your application
3. Configure in gorest.yaml
4. Migrations run automatically on startup

---

## [Unreleased]

### Planned for v1.1.0
- MySQL and SQLite migration files
- Additional import engines (Medium, Hashnode)
- Post tagging system
- Full-text search support
- Rate limiting per user

### Planned for v2.0.0
- Soft deletes
- Post revision history
- Media upload support
- SEO metadata (Open Graph, Twitter Cards)
- Multi-tenancy support
- Webhook notifications
- Redis caching layer

---

## Version History

- **v1.0.0** (2025-01-21) - Initial release with complete blog functionality and migration system

---

[1.0.0]: https://github.com/nicolasbonnici/gorest-blog-plugin/releases/tag/v1.0.0
[Unreleased]: https://github.com/nicolasbonnici/gorest-blog-plugin/compare/v1.0.0...HEAD
