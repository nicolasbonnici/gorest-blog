package blog

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/services"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func TestLoadPostsWithTranslationsAndMetrics_Integration(t *testing.T) {
	ctx := context.Background()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := createTestSchemaWithMetrics(ctx, db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	translationService := services.NewTranslationService(db)
	metricsService := services.NewMetricsService(db)

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

	if err := metricsService.InitializeMetrics(ctx, postID1); err != nil {
		t.Fatalf("Failed to initialize metrics for post 1: %v", err)
	}

	if err := metricsService.InitializeMetrics(ctx, postID2); err != nil {
		t.Fatalf("Failed to initialize metrics for post 2: %v", err)
	}

	if err := metricsService.IncrementViews(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment views for post 1: %v", err)
	}

	if err := metricsService.IncrementViews(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment views for post 1 again: %v", err)
	}

	if err := metricsService.IncrementLikes(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment likes for post 1: %v", err)
	}

	if err := metricsService.IncrementComments(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment comments for post 1: %v", err)
	}

	if err := metricsService.IncrementComments(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment comments for post 1 again: %v", err)
	}

	if err := metricsService.IncrementComments(ctx, postID1); err != nil {
		t.Fatalf("Failed to increment comments for post 1 third time: %v", err)
	}

	if err := metricsService.IncrementViews(ctx, postID2); err != nil {
		t.Fatalf("Failed to increment views for post 2: %v", err)
	}

	t.Run("Load posts with translations and metrics in single query", func(t *testing.T) {
		result, err := translationService.LoadPostsWithTranslations(ctx, 10, 0, true, nil, nil, "", crud.CountExact)
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
			if post.ID == postID1 {
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

				if post.Metrics == nil {
					t.Error("Expected metrics for post 1, got nil")
				} else {
					if post.Metrics.Views != 2 {
						t.Errorf("Expected 2 views for post 1, got %d", post.Metrics.Views)
					}
					if post.Metrics.Likes != 1 {
						t.Errorf("Expected 1 like for post 1, got %d", post.Metrics.Likes)
					}
					if post.Metrics.Comments != 3 {
						t.Errorf("Expected 3 comments for post 1, got %d", post.Metrics.Comments)
					}
				}
			}

			if post.ID == postID2 {
				if len(post.Translations) != 1 {
					t.Errorf("Expected 1 translation for post 2, got %d", len(post.Translations))
				}

				if post.Metrics == nil {
					t.Error("Expected metrics for post 2, got nil")
				} else {
					if post.Metrics.Views != 1 {
						t.Errorf("Expected 1 view for post 2, got %d", post.Metrics.Views)
					}
					if post.Metrics.Likes != 0 {
						t.Errorf("Expected 0 likes for post 2, got %d", post.Metrics.Likes)
					}
					if post.Metrics.Comments != 0 {
						t.Errorf("Expected 0 comments for post 2, got %d", post.Metrics.Comments)
					}
				}
			}
		}
	})

	t.Run("Verify no N+1 queries - a fixed number of queries loads all data", func(t *testing.T) {
		result, err := translationService.LoadPostsWithTranslations(ctx, 10, 0, false, nil, nil, "", crud.CountExact)
		if err != nil {
			t.Fatalf("LoadPostsWithTranslations failed: %v", err)
		}

		if len(result.Posts) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(result.Posts))
		}

		for _, post := range result.Posts {
			if post.Metrics == nil {
				t.Errorf("Post %s has nil metrics - metrics not loaded", post.ID)
			}

			if len(post.Translations) == 0 {
				t.Errorf("Post %s has no translations - translations not loaded", post.ID)
			}
		}
	})
}

func createTestSchemaWithMetrics(ctx context.Context, db database.Database) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS post (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			slug TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
			visual TEXT,
			published_at TEXT,
			updated_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS translations (
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
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS metrics (
			resource TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			name TEXT NOT NULL,
			value INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (resource, resource_id, name)
		)
	`)
	return err
}
