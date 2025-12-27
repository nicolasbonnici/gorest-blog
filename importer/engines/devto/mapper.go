package devto

import (
	"fmt"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

func MapPost(devtoArticle DevToArticle) engines.Post {
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
		ID:          fmt.Sprintf("%d", devtoArticle.ID),
		Title:       devtoArticle.Title,
		Content:     devtoArticle.BodyMarkdown,
		Slug:        devtoArticle.Slug,
		PublishedAt: publishedAt,
		UpdatedAt:   updatedAt,
		URL:         devtoArticle.URL,
		SourceID:    fmt.Sprintf("devto-%d", devtoArticle.ID),
	}
}

func MapPosts(devtoArticles []DevToArticle) []engines.Post {
	posts := make([]engines.Post, 0, len(devtoArticles))
	for _, da := range devtoArticles {
		posts = append(posts, MapPost(da))
	}
	return posts
}
