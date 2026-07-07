package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260401000001000",
		"add_post_status_published_index",
		addPostStatusPublishedIndexUp,
		addPostStatusPublishedIndexDown,
	)
}

// The public list endpoint filters unprivileged reads with
// status = 'published' AND published_at <= now. The single-column status index
// is too coarse (most rows are published); a composite (status, published_at)
// lets the planner range-scan just the visible, already-live posts.
func addPostStatusPublishedIndexUp(ctx context.Context, db database.Database) error {
	return migrations.CreateIndex(ctx, db, "idx_post_status_published", "post", "status, published_at")
}

func addPostStatusPublishedIndexDown(ctx context.Context, db database.Database) error {
	return migrations.DropIndex(ctx, db, "idx_post_status_published", "post")
}
