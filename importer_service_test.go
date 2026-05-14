package blog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest-blog/importer/engines"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func TestSyncMode_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		mode     importer.SyncMode
		expected bool
	}{
		{"local-wins is valid", importer.SyncModeLocalWins, true},
		{"remote-wins is valid", importer.SyncModeRemoteWins, true},
		{"import-only is valid", importer.SyncModeImportOnly, true},
		{"invalid mode", importer.SyncMode("invalid"), false},
		{"empty mode", importer.SyncMode(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mode.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSyncResult_Success(t *testing.T) {
	result := &importer.SyncResult{
		LocalCreated:  2,
		LocalUpdated:  3,
		RemoteCreated: 1,
		RemoteUpdated: 4,
		Skipped:       5,
	}

	expected := 10
	if got := result.Success(); got != expected {
		t.Errorf("Success() = %v, want %v", got, expected)
	}
}

func TestSyncResult_String(t *testing.T) {
	result := &importer.SyncResult{
		LocalCreated:  1,
		LocalUpdated:  2,
		RemoteCreated: 3,
		RemoteUpdated: 4,
		Skipped:       5,
		Errors:        []importer.SyncError{{PostSlug: "test", Operation: "test", Error: nil}},
	}

	str := result.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	expected := "Sync completed: 1 local created, 2 local updated, 3 remote created, 4 remote updated, 5 skipped, 1 errors"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestPostToModel_PublishedPost(t *testing.T) {
	service := &ImporterService{}
	post := importer.Post{
		Title:       "Test Post",
		Content:     "Test content",
		Slug:        "test-post",
		PublishedAt: "2024-01-01T00:00:00Z",
	}

	model := service.postToModel(post, "user-123")

	if model.Slug != "test-post" {
		t.Errorf("Expected slug 'test-post', got %s", model.Slug)
	}

	if model.Status != types.PostStatusPublished {
		t.Errorf("Expected status %s, got %s", types.PostStatusPublished, model.Status)
	}

	if model.PublishedAt == nil {
		t.Error("Expected PublishedAt to be set for published post")
	}
}

func TestPostToModel_DraftPost(t *testing.T) {
	service := &ImporterService{}
	post := importer.Post{
		Title:       "Draft Post",
		Content:     "Draft content",
		Slug:        "draft-post",
		PublishedAt: "",
	}

	model := service.postToModel(post, "user-123")

	if model.Status != types.PostStatusDrafted {
		t.Errorf("Expected status %s, got %s", types.PostStatusDrafted, model.Status)
	}

	if model.PublishedAt != nil {
		t.Error("Expected PublishedAt to be nil for draft post")
	}
}

func TestPostToModel_GeneratesSlug(t *testing.T) {
	service := &ImporterService{}
	post := importer.Post{
		Title:   "Post Without Slug",
		Content: "Content",
		Slug:    "",
	}

	model := service.postToModel(post, "user-123")

	if model.Slug == "" {
		t.Error("Expected slug to be generated from title")
	}

	expected := "post-without-slug"
	if model.Slug != expected {
		t.Errorf("Expected slug %s, got %s", expected, model.Slug)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Test_Post_Title", "test-post-title"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"CamelCaseTitle", "camelcasetitle"},
		{"Already-Slugified", "already-slugified"},
		{"Trailing-Dashes--", "trailing-dashes"},
		{"--Leading-Dashes", "leading-dashes"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestImportOptions_DefaultSyncMode(t *testing.T) {
	opts := importer.ImportOptions{
		Source: "devto",
		UserID: "user-123",
	}

	if opts.SyncMode == "" {
		opts.SyncMode = importer.SyncModeLocalWins
	}

	if opts.SyncMode != importer.SyncModeLocalWins {
		t.Errorf("Expected default sync mode to be local-wins, got %s", opts.SyncMode)
	}
}

func TestImportComment_HasPublishedStatus(t *testing.T) {
	ctx := context.Background()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec(ctx, `CREATE TABLE comment (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		commentable_id TEXT NOT NULL,
		commentable TEXT NOT NULL,
		parent_id TEXT,
		content TEXT NOT NULL,
		updated_at TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		status TEXT NOT NULL DEFAULT 'awaiting',
		ip_address TEXT,
		user_agent TEXT,
		remote_source_id TEXT,
		remote_source TEXT
	)`)
	if err != nil {
		t.Fatalf("Failed to create comment table: %v", err)
	}

	service := NewImporterService(db, nil)

	postID := uuid.New().String()
	userID := uuid.New().String()
	comment := engines.Comment{
		ID:        "devto-comment-999",
		Content:   "Imported comment from dev.to",
		CreatedAt: "2024-06-01T10:00:00Z",
	}

	count, err := service.importComment(ctx, postID, comment, nil, userID)
	if err != nil {
		t.Fatalf("importComment failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 comment imported, got %d", count)
	}

	var status string
	row := db.QueryRow(ctx, "SELECT status FROM comment WHERE commentable_id = ?", postID)
	if err := row.Scan(&status); err != nil {
		t.Fatalf("Failed to query imported comment status: %v", err)
	}
	if status != "published" {
		t.Errorf("Expected imported comment status 'published', got %q", status)
	}
}
