package blog

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
)

// IncrementViewCount increments the view count metric for a post.
// This should be called asynchronously from the GET post handler to avoid blocking the response.
// Deprecated: Use MetricsService.IncrementViews instead
func IncrementViewCount(ctx context.Context, db database.Database, postID string) error {
	service := NewMetricsService(db)
	return service.IncrementViews(ctx, postID)
}
