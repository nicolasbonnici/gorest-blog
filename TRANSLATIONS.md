# Post Translations

⚠️ **REQUIRED FEATURE**: As of v2.0.0, translations are REQUIRED for all blog posts. Post titles and content are stored exclusively in the translatable table.

This document describes the translation system in the gorest-blog plugin. The translation system is powered by the [gorest-translatable](https://github.com/nicolasbonnici/gorest-translatable) plugin and stores all blog post titles and content in multiple languages.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [JSON Structure](#json-structure)
- [API Usage](#api-usage)
- [Code Examples](#code-examples)
- [Security](#security)
- [Database Schema](#database-schema)

## Overview

⚠️ **Breaking Change**: As of v2.0.0, the `post` table no longer has `title` and `content` columns. All post content is stored in the `translatable` table.

The translation system enables you to:

- Store post titles and content in multiple languages
- Create posts with translations in multiple locales at once
- Update specific locale translations independently
- Automatically include all translations in GET responses
- Automatically sanitize content to prevent XSS attacks
- Support for PostgreSQL, MySQL, and SQLite

### Key Features

- **Primary Storage**: Post titles and content are stored exclusively in translations (required)
- **Atomic Storage**: Each locale's title and content are stored together in a single JSON record
- **Database-Level JSON Validation**: PostgreSQL (JSONB) and MySQL (JSON) provide native JSON validation
- **XSS Protection**: All content (title AND content) is automatically HTML-escaped before storage
- **Ownership Validation**: Users can only manage their own translations
- **Locale Validation**: Only configured locales are accepted
- **Always Included**: Translations are automatically fetched and included in all GET /posts responses

## Configuration

⚠️ **REQUIRED**: The `translatable` plugin is REQUIRED for the blog plugin to function. The `translatable` plugin must be loaded before the `blog` plugin.

### Example Configuration

```yaml
plugins:
  # Translatable plugin is REQUIRED (must be loaded first)
  - name: translatable
    enabled: true
    config:
      allowed_types: ["post"]  # Enable post translations (REQUIRED)
      supported_locales: ["en", "fr", "es", "de", "it", "pt"]
      default_locale: "en"
      max_content_length: 102400  # 100KB max JSON size
      pagination_limit: 20
      max_pagination_limit: 1000

  # Blog plugin requires translatable
  - name: blog
    enabled: true
    config:
      pagination_limit: 10
      max_pagination_limit: 1000
      enable_importer: true
```

### Configuration Options

**Translatable Plugin (REQUIRED):**

- `allowed_types`: MUST include `"post"` for blog plugin to work
- `supported_locales`: List of locale codes (e.g., "en", "fr", "es") - used for validation
- `default_locale`: Default locale for the application
- `max_content_length`: Maximum size of JSON content in bytes
- `pagination_limit`: Default number of results per page
- `max_pagination_limit`: Maximum allowed pagination limit

## JSON Structure

Each translation stores both the title and content in a single JSON structure:

```json
{
  "title": "My Blog Post",
  "content": "This is the post content..."
}
```

### Example Translation Record

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "translatable_id": "post-uuid",
  "translatable": "post",
  "locale": "fr",
  "content": {
    "title": "Mon Article de Blog",
    "content": "Ceci est le contenu de l'article..."
  },
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T15:45:00Z"
}
```

### Database Storage

- **PostgreSQL**: Content stored as JSONB (supports indexing and queries)
- **MySQL**: Content stored as JSON (native JSON type with validation)
- **SQLite**: Content stored as TEXT (JSON functions available)

## API Usage

⚠️ **API Change**: As of v2.0.0, you create and update translations through the `/posts` endpoint, not the `/translations` endpoint.

### Authentication

All post endpoints require authentication. Include your JWT token in the `Authorization` header:

```bash
Authorization: Bearer <your-jwt-token>
```

### Create Post with Translations

Create a new post with translations in one or more locales:

```bash
POST /api/posts
Content-Type: application/json
Authorization: Bearer <token>

{
  "slug": "my-blog-post",
  "status": "published",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "This is the post content..."
    },
    "fr": {
      "title": "Mon Article de Blog",
      "content": "Ceci est le contenu de l'article..."
    },
    "es": {
      "title": "Mi Artículo de Blog",
      "content": "Este es el contenido del artículo..."
    }
  }
}
```

**Response (201 Created):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "userId": "user-uuid",
  "slug": "my-blog-post",
  "status": "published",
  "publishedAt": "2024-01-15T10:00:00Z",
  "createdAt": "2024-01-15T10:00:00Z",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "This is the post content..."
    },
    "fr": {
      "title": "Mon Article de Blog",
      "content": "Ceci est le contenu de l'article..."
    },
    "es": {
      "title": "Mi Artículo de Blog",
      "content": "Este es el contenido del artículo..."
    }
  }
}
```

### Get Post with Translations

Retrieve a post - translations are ALWAYS included automatically:

```bash
GET /api/posts/<post-id>
Authorization: Bearer <token>
```

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "userId": "user-uuid",
  "slug": "my-blog-post",
  "status": "published",
  "publishedAt": "2024-01-15T10:00:00Z",
  "createdAt": "2024-01-15T10:00:00Z",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "This is the post content..."
    },
    "fr": {
      "title": "Mon Article de Blog",
      "content": "Ceci est le contenu en français..."
    },
    "es": {
      "title": "Mi Artículo de Blog",
      "content": "Este es el contenido en español..."
    }
  }
}
```

**Note:** The `translations` field is ALWAYS included in the response. There is no need for a query parameter.

### List Posts with Translations

List posts - translations are ALWAYS included automatically:

```bash
GET /api/posts
Authorization: Bearer <token>
```

Each post in the collection will include its translations.

### Update Post Translation

Update a translation for a specific locale:

```bash
PUT /api/posts/<post-id>
Content-Type: application/json
Authorization: Bearer <token>

{
  "locale": "fr",
  "title": "Titre Mis à Jour",
  "content": "Contenu mis à jour..."
}
```

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "slug": "my-blog-post",
  "status": "published",
  "publishedAt": "2024-01-15T10:00:00Z",
  "updatedAt": "2024-01-15T11:00:00Z",
  "translations": {
    "en": {
      "title": "My Blog Post",
      "content": "This is the post content..."
    },
    "fr": {
      "title": "Titre Mis à Jour",
      "content": "Contenu mis à jour..."
    },
    "es": {
      "title": "Mi Artículo de Blog",
      "content": "Este es el contenido del artículo..."
    }
  }
}
```

**Note:** Only the specified locale is updated. Other locales remain unchanged.

### Delete Post

Delete a post and ALL its translations:

```bash
DELETE /api/posts/<post-id>
Authorization: Bearer <token>
```

**Response (204 No Content)**

**Warning:** This deletes the post AND all its translations in all locales.

## Code Examples

### Using TranslationService

The `TranslationService` provides a convenient Go API for managing translations:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nicolasbonnici/gorest-blog"
    "github.com/nicolasbonnici/gorest/database"
)

func main() {
    // Initialize database connection
    db, err := database.Connect(/* your config */)
    if err != nil {
        log.Fatal(err)
    }

    // Create translation service
    service := blog.NewTranslationService(db)

    ctx := context.Background()
    postID := "550e8400-e29b-41d4-a716-446655440000"

    // Create multiple translations at once (recommended for new posts)
    translations := map[string]*blog.PostTranslationContent{
        "en": {
            Title:   "My Blog Post",
            Content: "This is the post content...",
        },
        "fr": {
            Title:   "Mon Article de Blog",
            Content: "Ceci est le contenu de l'article...",
        },
        "es": {
            Title:   "Mi Artículo de Blog",
            Content: "Este es el contenido del artículo...",
        },
    }

    err = service.CreateTranslations(ctx, postID, translations, nil)
    if err != nil {
        log.Printf("Failed to create translations: %v", err)
    }

    // Get a specific translation
    translation, err := service.GetTranslation(ctx, postID, "fr")
    if err != nil {
        log.Printf("Failed to get translation: %v", err)
    } else {
        fmt.Printf("Title: %s\n", translation.Title)
        fmt.Printf("Content: %s\n", translation.Content)
    }

    // List all translations for a post
    allTranslations, err := service.ListTranslations(ctx, postID)
    if err != nil {
        log.Printf("Failed to list translations: %v", err)
    } else {
        for locale, trans := range allTranslations {
            fmt.Printf("[%s] %s\n", locale, trans.Title)
        }
    }

    // Update a specific locale translation
    err = service.UpdateTranslation(ctx, postID, "fr",
        "Titre Mis à Jour",
        "Contenu mis à jour...",
        nil)
    if err != nil {
        log.Printf("Failed to update translation: %v", err)
    }

    // Delete a single locale translation
    err = service.DeleteTranslation(ctx, postID, "fr", nil)
    if err != nil {
        log.Printf("Failed to delete translation: %v", err)
    }

    // Delete ALL translations for a post
    err = service.DeleteAllTranslations(ctx, postID)
    if err != nil {
        log.Printf("Failed to delete all translations: %v", err)
    }
}
```

### Working with PostTranslationContent

```go
package main

import (
    "fmt"
    "log"

    "github.com/nicolasbonnici/gorest-blog"
)

func main() {
    // Create translation content
    content := &blog.PostTranslationContent{
        Title:   "My Blog Post <script>alert('xss')</script>",
        Content: "This is the post content with <script>alert('xss')</script>",
    }

    // Validate
    if err := content.Validate(); err != nil {
        log.Fatal(err)
    }

    // Sanitize (prevents XSS on BOTH title and content)
    content.Sanitize()
    fmt.Printf("Sanitized title: %s\n", content.Title)
    // Output: Sanitized title: My Blog Post &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;
    fmt.Printf("Sanitized content: %s\n", content.Content)
    // Output: Sanitized content: This is the post content with &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;

    // Serialize to JSON
    jsonStr, err := content.ToJSON()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("JSON: %s\n", jsonStr)

    // Parse from JSON
    parsed, err := blog.ParsePostTranslationContent(jsonStr)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Parsed title: %s\n", parsed.Title)
}
```

## Security

### XSS Protection

⚠️ **Important**: As of v2.0.0, BOTH title AND content are automatically HTML-escaped before being stored in the database. This prevents Cross-Site Scripting (XSS) attacks.

**Example:**

```go
titleInput := "<script>alert('xss')</script>"
contentInput := "<img src=x onerror=alert('xss')>"

// After sanitization:
titleOutput := "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"
contentOutput := "&lt;img src=x onerror=alert(&#39;xss&#39;)&gt;"
```

**Previous Behavior (v1.x):** Only content was escaped, titles were not.
**Current Behavior (v2.0.0+):** Both title and content are escaped.

### Ownership Validation

The translation service validates that users can only modify their own translations. The `user_id` field in the database tracks ownership.

### Locale Validation

Only locales configured in `supported_locales` are accepted. Invalid locales are rejected with an error.

### Content Length Limits

The `max_content_length` configuration option limits the size of JSON content to prevent abuse.

### Database-Level Validation

- **PostgreSQL JSONB**: Invalid JSON is rejected at the database level
- **MySQL JSON**: Invalid JSON is rejected at the database level
- **SQLite**: JSON validation is performed at the application level

## Database Schema

The `translatable` table stores all translations:

### PostgreSQL

```sql
CREATE TABLE translatable (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    translatable_id UUID NOT NULL,
    translatable TEXT NOT NULL,
    locale TEXT NOT NULL DEFAULT 'en',
    content JSONB NOT NULL,
    updated_at TIMESTAMP(0) WITH TIME ZONE,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(translatable_id, translatable, locale)
);
```

### MySQL

```sql
CREATE TABLE translatable (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    translatable_id CHAR(36) NOT NULL,
    translatable VARCHAR(255) NOT NULL,
    locale VARCHAR(10) NOT NULL DEFAULT 'en',
    content JSON NOT NULL,
    updated_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE KEY unique_translation (translatable_id, translatable, locale)
);
```

### SQLite

```sql
CREATE TABLE translatable (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    translatable_id TEXT NOT NULL,
    translatable TEXT NOT NULL,
    locale TEXT NOT NULL DEFAULT 'en',
    content TEXT NOT NULL,
    updated_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(translatable_id, translatable, locale)
);
```

### Key Constraints

- **UNIQUE(translatable_id, translatable, locale)**: Ensures one translation per locale per post
- **Foreign Key (user_id)**: Links translations to users
- **NOT NULL (content)**: Requires content for all translations

## Best Practices

1. **Always validate content** before creating/updating translations
2. **Use the TranslationService** instead of direct database queries
3. **Configure appropriate max_content_length** based on your needs
4. **Enable only required locales** in supported_locales
5. **Implement proper error handling** when working with translations
6. **Consider translation completeness** in your UI (show which locales are available)

## Troubleshooting

### "translation not found" Error

- Verify the post ID exists
- Check the locale is in supported_locales
- Ensure you're querying with the correct parameters

### "validation failed" Error

- Check that title and content are not empty
- Verify content length is within max_content_length
- Ensure locale is in supported_locales

### "post not found" Error

- Verify the post exists in the database
- Check the post ID is a valid UUID

### JSON Parse Errors

- Ensure content is valid JSON
- Check for proper escaping of special characters
- Verify database driver supports JSON type (PostgreSQL JSONB, MySQL JSON)

## Future Enhancements

Potential future improvements:

- Implement auto-translation via Google Translate or DeepL API
- Add bulk import/export functionality for translations
- Create translation completion tracking dashboard
- Implement fallback to default locale if translation is missing
- Add translation history and versioning

## Support

For issues or questions:

- GitHub Issues: [gorest-blog](https://github.com/nicolasbonnici/gorest-blog/issues)
- Plugin Issues: [gorest-translatable](https://github.com/nicolasbonnici/gorest-translatable/issues)
