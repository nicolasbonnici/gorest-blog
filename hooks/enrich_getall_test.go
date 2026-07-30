package hooks

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	taxonomy "github.com/nicolasbonnici/gorest-taxonomy"
	taxonomymigrations "github.com/nicolasbonnici/gorest-taxonomy/migrations"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"

	"github.com/nicolasbonnici/gorest-blog/models"
)

// countingDB records every statement so a test can assert on the number of
// round-trips rather than only on the enriched result.
type countingDB struct {
	database.Database
	queries []string
}

func (d *countingDB) Query(ctx context.Context, q string, args ...any) (database.Rows, error) {
	d.queries = append(d.queries, q)
	return d.Database.Query(ctx, q, args...)
}

func (d *countingDB) QueryRow(ctx context.Context, q string, args ...any) database.Row {
	d.queries = append(d.queries, q)
	return d.Database.QueryRow(ctx, q, args...)
}

func (d *countingDB) selectCount() int {
	n := 0
	for _, q := range d.queries {
		if strings.Contains(strings.ToUpper(q), "SELECT") {
			n++
		}
	}
	return n
}

// setupTaxonomySchema applies the taxonomy plugin's own migrations so the test
// runs against the real schema rather than a hand-rolled approximation.
func setupTaxonomySchema(t *testing.T, ctx context.Context, db database.Database) {
	t.Helper()

	source := taxonomymigrations.GetMigrations()
	migs, err := source.Migrations()
	if err != nil {
		t.Fatalf("failed to load taxonomy migrations: %v", err)
	}
	for _, m := range migs {
		if err := m.ExecuteUp(ctx, db); err != nil {
			t.Fatalf("failed to apply migration %s: %v", m.FullName(), err)
		}
	}
}

func TestEnrichGetAll_BatchesTaxonomyLookups(t *testing.T) {
	ctx := context.Background()

	base, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer func() { _ = base.Close() }()

	setupTaxonomySchema(t, ctx, base)

	const postCount = 20
	posts := make([]*models.Post, 0, postCount)
	now := "2026-01-01T00:00:00Z"

	categoryID := uuid.New()
	tagID := uuid.New()
	if _, err := base.Exec(ctx,
		`INSERT INTO categories (id, name, slug, created_at) VALUES (?, ?, ?, ?)`,
		categoryID, "Go", "go", now); err != nil {
		t.Fatalf("failed to insert category: %v", err)
	}
	if _, err := base.Exec(ctx,
		`INSERT INTO tags (id, name, slug, created_at) VALUES (?, ?, ?, ?)`,
		tagID, "perf", "perf", now); err != nil {
		t.Fatalf("failed to insert tag: %v", err)
	}

	// Ids bind as uuid.UUID, matching what the service passes to its IN clause.
	for range postCount {
		postID := uuid.New()
		posts = append(posts, &models.Post{ID: postID.String()})

		if _, err := base.Exec(ctx,
			`INSERT INTO category_resources (category_id, resource, resource_id) VALUES (?, ?, ?)`,
			categoryID, PostResourceType, postID); err != nil {
			t.Fatalf("failed to link category: %v", err)
		}
		if _, err := base.Exec(ctx,
			`INSERT INTO tag_resources (tag_id, resource, resource_id) VALUES (?, ?, ?)`,
			tagID, PostResourceType, postID); err != nil {
			t.Fatalf("failed to link tag: %v", err)
		}
	}

	db := &countingDB{Database: base}
	h := &PostHooks{}
	cfg := taxonomy.DefaultConfig()
	h.SetTaxonomyService(taxonomy.NewTaxonomyService(db, &cfg))

	if err := h.EnrichGetAll(ctx, nil, posts); err != nil {
		t.Fatalf("EnrichGetAll failed: %v", err)
	}

	// One lookup for categories, one for tags — not two per post.
	if got := db.selectCount(); got != 2 {
		t.Errorf("expected 2 queries for %d posts, got %d", postCount, got)
	}

	for i, post := range posts {
		if len(post.Categories) != 1 {
			t.Errorf("post %d: expected 1 category, got %d", i, len(post.Categories))
		}
		if len(post.Tags) != 1 {
			t.Errorf("post %d: expected 1 tag, got %d", i, len(post.Tags))
		}
	}
}

func TestEnrichGetAll_NoPostsIssuesNoQueries(t *testing.T) {
	ctx := context.Background()

	base, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer func() { _ = base.Close() }()

	setupTaxonomySchema(t, ctx, base)

	db := &countingDB{Database: base}
	h := &PostHooks{}
	cfg := taxonomy.DefaultConfig()
	h.SetTaxonomyService(taxonomy.NewTaxonomyService(db, &cfg))

	if err := h.EnrichGetAll(ctx, nil, nil); err != nil {
		t.Fatalf("EnrichGetAll failed: %v", err)
	}

	if got := db.selectCount(); got != 0 {
		t.Errorf("expected no queries for an empty page, got %d", got)
	}
}

func TestEnrichGetAll_WithoutTaxonomyServiceIsNoOp(t *testing.T) {
	h := &PostHooks{}
	posts := []*models.Post{{ID: uuid.New().String()}}

	if err := h.EnrichGetAll(context.Background(), nil, posts); err != nil {
		t.Fatalf("EnrichGetAll failed: %v", err)
	}

	if posts[0].Categories != nil || posts[0].Tags != nil {
		t.Error("expected no enrichment without a taxonomy service")
	}
}
