package blog

import (
	"context"
	"testing"

	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

// TestTruncatePosts_RemovesPostsAndTranslations verifies the query-builder port
// of truncatePosts still deletes every post row and the post translations, and
// leaves translations of other translatable types untouched.
func TestTruncatePosts_RemovesPostsAndTranslations(t *testing.T) {
	ctx := context.Background()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(ctx, `CREATE TABLE post (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		slug TEXT
	)`); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE translations (
		id TEXT PRIMARY KEY,
		translatable TEXT NOT NULL,
		translatable_id TEXT NOT NULL,
		locale TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create translations: %v", err)
	}

	if _, err := db.Exec(ctx, `INSERT INTO post (id, user_id, slug) VALUES ('p1','u1','a'),('p2','u1','b')`); err != nil {
		t.Fatalf("seed posts: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO translations (id, translatable, translatable_id, locale) VALUES
		('t1','post','p1','en'),
		('t2','post','p2','fr'),
		('t3','page','x1','en')`); err != nil {
		t.Fatalf("seed translations: %v", err)
	}

	service := NewImporterService(db, nil)
	if err := service.truncatePosts(ctx); err != nil {
		t.Fatalf("truncatePosts: %v", err)
	}

	var postCount int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM post").Scan(&postCount); err != nil {
		t.Fatalf("count posts: %v", err)
	}
	if postCount != 0 {
		t.Fatalf("expected 0 posts after truncate, got %d", postCount)
	}

	var postTranslations int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM translations WHERE translatable = 'post'").Scan(&postTranslations); err != nil {
		t.Fatalf("count post translations: %v", err)
	}
	if postTranslations != 0 {
		t.Fatalf("expected 0 post translations after truncate, got %d", postTranslations)
	}

	var otherTranslations int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM translations WHERE translatable = 'page'").Scan(&otherTranslations); err != nil {
		t.Fatalf("count other translations: %v", err)
	}
	if otherTranslations != 1 {
		t.Fatalf("expected non-post translations preserved, got %d", otherTranslations)
	}
}
