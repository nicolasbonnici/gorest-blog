package devto

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

func MapPost(devtoArticle DevToArticle, comments []DevToComment) engines.Post {
	// Only set PublishedAt if it's not a zero time
	publishedAt := ""
	if !devtoArticle.PublishedAt.IsZero() {
		publishedAt = devtoArticle.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	updatedAt := ""
	if !devtoArticle.EditedAt.IsZero() {
		updatedAt = devtoArticle.EditedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return engines.Post{
		ID:            fmt.Sprintf("%d", devtoArticle.ID),
		Title:         devtoArticle.Title,
		Content:       devtoArticle.BodyMarkdown,
		Slug:          devtoArticle.Slug,
		PublishedAt:   publishedAt,
		UpdatedAt:     updatedAt,
		URL:           devtoArticle.URL,
		SourceID:      fmt.Sprintf("devto-%d", devtoArticle.ID),
		LikesCount:    devtoArticle.PublicReactions,
		CommentsCount: devtoArticle.CommentsCount,
		ViewsCount:    0, // dev.to doesn't expose view counts via API
		Comments:      MapComments(comments, ""),
	}
}

func MapComments(devtoComments []DevToComment, parentID string) []engines.Comment {
	comments := make([]engines.Comment, 0, len(devtoComments))
	for _, dc := range devtoComments {
		comments = append(comments, MapComment(dc, parentID))
	}
	return comments
}

func MapComment(devtoComment DevToComment, parentID string) engines.Comment {
	createdAt := ""
	if !devtoComment.CreatedAt.IsZero() {
		createdAt = devtoComment.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	// Convert HTML content to plain text (basic conversion)
	content := htmlToMarkdown(devtoComment.BodyHTML)

	comment := engines.Comment{
		ID:        devtoComment.ID,
		Content:   content,
		CreatedAt: createdAt,
		ParentID:  parentID,
		Author: engines.CommentAuthor{
			Name:     devtoComment.User.Name,
			Username: devtoComment.User.Username,
		},
	}

	// Recursively map children comments
	if len(devtoComment.Children) > 0 {
		comment.Children = MapComments(devtoComment.Children, comment.ID)
	}

	return comment
}

// Basic HTML to Markdown conversion
func htmlToMarkdown(htmlContent string) string {
	// Unescape HTML entities
	content := html.UnescapeString(htmlContent)

	// Remove HTML tags (basic approach)
	content = stripHTMLTags(content)

	// Clean up whitespace
	content = strings.TrimSpace(content)

	return content
}

func stripHTMLTags(content string) string {
	// Remove all HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(content, "")
}
