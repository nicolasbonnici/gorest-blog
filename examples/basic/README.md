# GoREST Blog Plugin - Basic Example

This example demonstrates how to use the GoREST Blog Plugin in a minimal application.

## Setup

1. **Install dependencies**:
```bash
go mod init myapp
go get github.com/nicolasbonnici/gorest-blog-plugin
```

2. **Configure environment**:
```bash
cp .env.example .env
# Edit .env with your database credentials
```

3. **Run the application**:
```bash
go run main.go
```

The server will start on `http://localhost:8000` and automatically run migrations.

## What's Included

This example configures:
- **Auth Plugin**: User authentication with JWT
- **Blog Plugin**: Posts, comments, and likes with importer
- **Standard Middleware**: CORS, rate limiting, logging, etc.
- **Auto-Migrations**: Database schema created on startup

## API Usage

### 1. Create a User (via database or seed)

```sql
INSERT INTO users (firstname, lastname, email, password)
VALUES ('John', 'Doe', 'john@example.com', '$2a$10$xZybcXcww7epzFX6d6yr1uWKJvnqs7cEySXCKDYlBN1frJeUswGla');
-- Password: "password"
```

### 2. Login

```bash
curl -X POST http://localhost:8000/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "password"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "uuid",
    "email": "john@example.com",
    "firstname": "John",
    "lastname": "Doe"
  }
}
```

### 3. Create a Post

```bash
curl -X POST http://localhost:8000/posts \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My First Post",
    "slug": "my-first-post",
    "content": "This is the content of my first post.",
    "status": "published"
  }'
```

### 4. Get All Posts

```bash
# Public access - only published posts
curl http://localhost:8000/posts

# Authenticated - all posts
curl -H "Authorization: Bearer <your-token>" http://localhost:8000/posts
```

### 5. Import from Dev.to (if enabled)

```bash
curl -X POST http://localhost:8000/api/import/devto \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "devto_username",
    "user_id": "<your-user-uuid>",
    "update_existing": false
  }'
```

## Testing Migrations

```bash
# Check migration status
gorest migrate status

# Run migrations manually
gorest migrate up

# Rollback last migration
gorest migrate down
```

## Next Steps

- Explore the [full documentation](../../README.md)
- Add custom hooks
- Extend with additional import engines
- Customize the schema
