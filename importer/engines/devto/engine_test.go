package devto

import (
	"context"
	"testing"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

func TestEngine_Name(t *testing.T) {
	engine := NewEngine()
	if engine.Name() != "devto" {
		t.Errorf("Expected engine name to be 'devto', got %s", engine.Name())
	}
}

func TestEngine_CreatePost_RequiresAPIKey(t *testing.T) {
	engine := NewEngine()
	post := engines.Post{
		Title:       "Test Post",
		Content:     "Test content",
		Slug:        "test-post",
		PublishedAt: "2024-01-01T00:00:00Z",
	}

	_, err := engine.CreatePost(context.Background(), "", post)
	if err == nil {
		t.Error("Expected error when API key is empty, got nil")
	}

	if err.Error() != "API key is required for creating posts on dev.to" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestEngine_UpdatePost_RequiresAPIKey(t *testing.T) {
	engine := NewEngine()
	post := engines.Post{
		Title:       "Test Post",
		Content:     "Test content",
		Slug:        "test-post",
		PublishedAt: "2024-01-01T00:00:00Z",
	}

	err := engine.UpdatePost(context.Background(), "", "123", post)
	if err == nil {
		t.Error("Expected error when API key is empty, got nil")
	}

	if err.Error() != "API key is required for updating posts on dev.to" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestEngine_UpdatePost_InvalidRemoteID(t *testing.T) {
	engine := NewEngine()
	post := engines.Post{
		Title:   "Test Post",
		Content: "Test content",
		Slug:    "test-post",
	}

	err := engine.UpdatePost(context.Background(), "fake-api-key", "invalid-id", post)
	if err == nil {
		t.Error("Expected error for invalid remote ID, got nil")
	}
}

func TestCreateArticlePayload_Published(t *testing.T) {
	payload := CreateArticlePayload{
		Title:        "Test Article",
		BodyMarkdown: "# Test Content",
		Published:    true,
		Tags:         []string{"go", "testing"},
	}

	if payload.Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got %s", payload.Title)
	}

	if !payload.Published {
		t.Error("Expected Published to be true")
	}

	if len(payload.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(payload.Tags))
	}
}

func TestCreateArticlePayload_Draft(t *testing.T) {
	payload := CreateArticlePayload{
		Title:        "Draft Article",
		BodyMarkdown: "# Draft Content",
		Published:    false,
	}

	if payload.Published {
		t.Error("Expected Published to be false for draft")
	}
}

func TestDevToArticle_ArchivedField(t *testing.T) {
	tests := []struct {
		name     string
		article  DevToArticle
		expected bool
	}{
		{
			name: "non-archived article",
			article: DevToArticle{
				ID:       123,
				Title:    "Active Article",
				Archived: false,
			},
			expected: false,
		},
		{
			name: "archived article",
			article: DevToArticle{
				ID:       456,
				Title:    "Archived Article",
				Archived: true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.article.Archived != tt.expected {
				t.Errorf("Expected Archived=%v, got %v", tt.expected, tt.article.Archived)
			}
		})
	}
}
