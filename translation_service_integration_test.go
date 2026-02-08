package blog

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
	"github.com/nicolasbonnici/gorest/query"
)

func TestLoadPostsWithTranslations_Integration(t *testing.T) {
	ctx := context.Background()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := createTestSchema(ctx, db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	service := NewTranslationService(db)

	postID1 := uuid.New().String()
	postID2 := uuid.New().String()
	now := time.Now()

	if err := insertTestPost(ctx, db, postID1, "test-post-1", types.PostStatusPublished, &now); err != nil {
		t.Fatalf("Failed to insert test post 1: %v", err)
	}

	if err := insertTestPost(ctx, db, postID2, "test-post-2", types.PostStatusDrafted, nil); err != nil {
		t.Fatalf("Failed to insert test post 2: %v", err)
	}

	if err := insertTestTranslation(ctx, db, postID1, "en", "English Title 1", "English Content 1"); err != nil {
		t.Fatalf("Failed to insert en translation for post 1: %v", err)
	}

	if err := insertTestTranslation(ctx, db, postID1, "fr", "French Title 1", "French Content 1"); err != nil {
		t.Fatalf("Failed to insert fr translation for post 1: %v", err)
	}

	if err := insertTestTranslation(ctx, db, postID2, "en", "English Title 2", "English Content 2"); err != nil {
		t.Fatalf("Failed to insert en translation for post 2: %v", err)
	}

	t.Run("Load all posts with translations", func(t *testing.T) {
		result, err := service.LoadPostsWithTranslations(ctx, 10, 0, true, nil, nil)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if len(result.Posts) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(result.Posts))
		}

		if result.Total == nil {
			t.Error("Expected total count, got nil")
		} else if *result.Total != 2 {
			t.Errorf("Expected total count 2, got %d", *result.Total)
		}

		for _, post := range result.Posts {
			if post.Id == postID1 {
				if len(post.Translations) != 2 {
					t.Errorf("Expected 2 translations for post 1, got %d", len(post.Translations))
				}

				if trans, ok := post.Translations["en"]; !ok {
					t.Error("Expected 'en' translation for post 1")
				} else {
					if trans.Title != "English Title 1" {
						t.Errorf("Expected 'English Title 1', got %s", trans.Title)
					}
					if trans.Content != "English Content 1" {
						t.Errorf("Expected 'English Content 1', got %s", trans.Content)
					}
				}

				if trans, ok := post.Translations["fr"]; !ok {
					t.Error("Expected 'fr' translation for post 1")
				} else {
					if trans.Title != "French Title 1" {
						t.Errorf("Expected 'French Title 1', got %s", trans.Title)
					}
				}
			}

			if post.Id == postID2 {
				if len(post.Translations) != 1 {
					t.Errorf("Expected 1 translation for post 2, got %d", len(post.Translations))
				}

				if trans, ok := post.Translations["en"]; !ok {
					t.Error("Expected 'en' translation for post 2")
				} else {
					if trans.Title != "English Title 2" {
						t.Errorf("Expected 'English Title 2', got %s", trans.Title)
					}
				}
			}
		}
	})

	t.Run("Load posts with pagination", func(t *testing.T) {
		result, err := service.LoadPostsWithTranslations(ctx, 1, 0, true, nil, nil)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if len(result.Posts) != 1 {
			t.Errorf("Expected 1 post with limit, got %d", len(result.Posts))
		}

		if result.Total == nil || *result.Total != 2 {
			t.Errorf("Expected total count 2 regardless of limit, got %v", result.Total)
		}
	})

	t.Run("Load posts with filters", func(t *testing.T) {
		conditions := []query.Condition{
			query.Eq("p.status", string(types.PostStatusPublished)),
		}

		result, err := service.LoadPostsWithTranslations(ctx, 10, 0, true, conditions, nil)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if len(result.Posts) != 1 {
			t.Errorf("Expected 1 published post, got %d", len(result.Posts))
		}

		if result.Posts[0].Status != types.PostStatusPublished {
			t.Errorf("Expected published status, got %s", result.Posts[0].Status)
		}
	})

	t.Run("Load posts with ordering", func(t *testing.T) {
		orderBy := []crud.OrderByClause{
			{Column: "p.slug", Direction: query.ASC},
		}

		result, err := service.LoadPostsWithTranslations(ctx, 10, 0, false, nil, orderBy)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if len(result.Posts) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(result.Posts))
		}

		if result.Posts[0].Slug != "test-post-1" {
			t.Errorf("Expected first post slug 'test-post-1', got %s", result.Posts[0].Slug)
		}

		if result.Posts[1].Slug != "test-post-2" {
			t.Errorf("Expected second post slug 'test-post-2', got %s", result.Posts[1].Slug)
		}
	})

	t.Run("Load posts without count", func(t *testing.T) {
		result, err := service.LoadPostsWithTranslations(ctx, 10, 0, false, nil, nil)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if result.Total != nil {
			t.Errorf("Expected nil total when includeCount is false, got %v", result.Total)
		}
	})
}

func createTestSchema(ctx context.Context, db database.Database) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE post (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			slug TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
			published_at TEXT,
			updated_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		CREATE TABLE translations (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			translatable_id TEXT NOT NULL,
			translatable TEXT NOT NULL,
			locale TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT
		)
	`)
	return err
}

func insertTestPost(ctx context.Context, db database.Database, id, slug string, status types.PostStatus, publishedAt *time.Time) error {
	var pubAt interface{}
	if publishedAt != nil {
		pubAt = publishedAt.Format(time.RFC3339)
	}

	_, err := db.Exec(ctx, `
		INSERT INTO post (id, slug, status, published_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, slug, string(status), pubAt, time.Now().Format(time.RFC3339))
	return err
}

func insertTestTranslation(ctx context.Context, db database.Database, postID, locale, title, content string) error {
	translationContent := &PostTranslationContent{
		Title:   title,
		Content: content,
	}

	jsonContent, err := translationContent.ToJSON()
	if err != nil {
		return err
	}

	translationID := uuid.New().String()
	_, err = db.Exec(ctx, `
		INSERT INTO translations (id, translatable_id, translatable, locale, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, translationID, postID, TranslatableTypePost, locale, jsonContent, time.Now().Format(time.RFC3339))
	return err
}
