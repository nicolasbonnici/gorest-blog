package devto

import (
	"testing"
	"time"
)

func TestMapPost_HTMLEntityDecoding(t *testing.T) {
	// Dev.to returns markdown with HTML entities that need to be decoded
	article := DevToArticle{
		ID:    123,
		Title: "Test Article",
		Slug:  "test-article",
		// Content with HTML entities (as returned by Dev.to API)
		BodyMarkdown: "```go\npackage main\n\nimport &#34;fmt&#34;\n\nfunc main() {\n    fmt.Println(&#34;Hello&#34;)\n}\n```",
		PublishedAt:  time.Now(),
	}

	post := MapPost(article, []DevToComment{})

	// Verify HTML entities are decoded
	expectedContent := "```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"
	if post.Content != expectedContent {
		t.Errorf("HTML entities not properly decoded.\nExpected: %s\nGot: %s", expectedContent, post.Content)
	}
}

func TestMapPost_BasicFields(t *testing.T) {
	publishedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	article := DevToArticle{
		ID:              456,
		Title:           "My Article",
		Slug:            "my-article",
		BodyMarkdown:    "# Content",
		URL:             "https://dev.to/user/my-article",
		PublishedAt:     publishedAt,
		PublicReactions: 10,
		CommentsCount:   5,
	}

	post := MapPost(article, []DevToComment{})

	if post.ID != "456" {
		t.Errorf("Expected ID '456', got '%s'", post.ID)
	}
	if post.Title != "My Article" {
		t.Errorf("Expected title 'My Article', got '%s'", post.Title)
	}
	if post.Slug != "my-article" {
		t.Errorf("Expected slug 'my-article', got '%s'", post.Slug)
	}
	if post.Content != "# Content" {
		t.Errorf("Expected content '# Content', got '%s'", post.Content)
	}
	if post.URL != "https://dev.to/user/my-article" {
		t.Errorf("Expected URL 'https://dev.to/user/my-article', got '%s'", post.URL)
	}
	if post.LikesCount != 10 {
		t.Errorf("Expected likes count 10, got %d", post.LikesCount)
	}
	if post.CommentsCount != 5 {
		t.Errorf("Expected comments count 5, got %d", post.CommentsCount)
	}
	if post.ViewsCount != 0 {
		t.Errorf("Expected views count 0, got %d", post.ViewsCount)
	}
}

func TestMapPost_EmptyPublishedAt(t *testing.T) {
	article := DevToArticle{
		ID:           789,
		Title:        "Draft Article",
		Slug:         "draft",
		BodyMarkdown: "Content",
		// PublishedAt is zero time
	}

	post := MapPost(article, []DevToComment{})

	if post.PublishedAt != "" {
		t.Errorf("Expected empty PublishedAt for zero time, got '%s'", post.PublishedAt)
	}
}

func TestMapPost_WithComments(t *testing.T) {
	article := DevToArticle{
		ID:           100,
		Title:        "Article with Comments",
		Slug:         "article-comments",
		BodyMarkdown: "Content",
	}

	comments := []DevToComment{
		{
			ID:        "comment1",
			BodyHTML:  "<p>Great article!</p>",
			CreatedAt: time.Now(),
			User: DevToCommentAuthor{
				Name:     "John Doe",
				Username: "johndoe",
			},
		},
	}

	post := MapPost(article, comments)

	if len(post.Comments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(post.Comments))
	}
	if post.Comments[0].ID != "comment1" {
		t.Errorf("Expected comment ID 'comment1', got '%s'", post.Comments[0].ID)
	}
}
