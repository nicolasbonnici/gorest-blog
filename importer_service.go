package blog

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest-blog/importer/engines"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"
)

type ImporterService struct {
	crud     *crud.CRUD[Post]
	db       database.Database
	qb       *query.Builder
	reporter importer.ProgressReporter
}

func NewImporterService(db database.Database, reporter importer.ProgressReporter) *ImporterService {
	if reporter == nil {
		reporter = &importer.NoOpProgressReporter{}
	}
	return &ImporterService{
		crud:     crud.New[Post](db),
		db:       db,
		qb:       query.New(db.Dialect()),
		reporter: reporter,
	}
}

func (s *ImporterService) Import(ctx context.Context, opts importer.ImportOptions) (*importer.ImportResult, error) {
	if err := s.validateImportOptions(ctx, opts); err != nil {
		return nil, err
	}

	engine, ok := engines.Get(opts.Source)
	if !ok {
		return nil, fmt.Errorf("unknown engine: %s (available: %v)", opts.Source, engines.List())
	}

	posts, err := s.fetchPosts(ctx, engine, opts)
	if err != nil {
		return nil, err
	}

	if opts.Truncate && !opts.DryRun {
		if err := s.truncatePosts(ctx); err != nil {
			return nil, fmt.Errorf("failed to truncate posts: %w", err)
		}
	}

	return s.processPosts(ctx, posts, opts)
}

func (s *ImporterService) validateImportOptions(ctx context.Context, opts importer.ImportOptions) error {
	if opts.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	userExists, err := s.userExists(ctx, opts.UserID)
	if err != nil {
		return fmt.Errorf("failed to validate user: %w", err)
	}
	if !userExists {
		return fmt.Errorf("user_id '%s' does not exist", opts.UserID)
	}

	return nil
}

func (s *ImporterService) fetchPosts(ctx context.Context, engine engines.Engine, opts importer.ImportOptions) ([]importer.Post, error) {
	if opts.Username != "" {
		posts, err := engine.FetchByUsername(ctx, opts.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts by username: %w", err)
		}
		return posts, nil
	}

	if opts.ArticleURL != "" {
		post, err := engine.FetchByURL(ctx, opts.ArticleURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by URL: %w", err)
		}
		return []importer.Post{*post}, nil
	}

	if opts.ArticleID != "" {
		post, err := engine.FetchByID(ctx, opts.ArticleID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by ID: %w", err)
		}
		return []importer.Post{*post}, nil
	}

	return nil, fmt.Errorf("one of username, url, or id must be provided")
}

func (s *ImporterService) processPosts(ctx context.Context, posts []importer.Post, opts importer.ImportOptions) (*importer.ImportResult, error) {
	result := &importer.ImportResult{
		TotalFetched: len(posts),
		Errors:       make([]error, 0),
	}

	if s.reporter != nil {
		s.reporter.Start(len(posts), fmt.Sprintf("Importing %d posts from %s", len(posts), opts.Source))
	}

	for i, post := range posts {
		if s.reporter != nil {
			s.reporter.Update(i+1, fmt.Sprintf("Processing: %s", post.Title))
		}

		action, err := s.importPost(ctx, post, opts)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to import '%s': %w", post.Title, err))
			if s.reporter != nil {
				s.reporter.Error(err)
			}
			continue
		}

		s.updateResultCounts(result, action)

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	if s.reporter != nil {
		s.reporter.Finish(result.String())
	}

	return result, nil
}

func (s *ImporterService) updateResultCounts(result *importer.ImportResult, action string) {
	switch action {
	case "created":
		result.Created++
	case "updated":
		result.Updated++
	case "skipped":
		result.Skipped++
	}
}

func (s *ImporterService) importPost(ctx context.Context, post importer.Post, opts importer.ImportOptions) (string, error) {
	postModel := s.postToModel(post, opts.UserID)
	defaultLocale := "en"

	if opts.DryRun {
		return s.handleDryRun(ctx, postModel.Slug)
	}

	existing, err := s.findBySlug(ctx, postModel.Slug)
	if err == nil && existing != nil {
		return s.updateExistingPost(ctx, existing, postModel, post, defaultLocale, opts)
	}

	return s.createNewPost(ctx, postModel, post, defaultLocale, opts)
}

func (s *ImporterService) handleDryRun(ctx context.Context, slug string) (string, error) {
	existing, err := s.findBySlug(ctx, slug)
	if err == nil && existing != nil {
		return "updated", nil
	}
	return "created", nil
}

func (s *ImporterService) updateExistingPost(ctx context.Context, existing *Post, postModel Post, post importer.Post, defaultLocale string, opts importer.ImportOptions) (string, error) {
	if err := s.crud.Update(ctx, existing.Id, postModel); err != nil {
		return "", fmt.Errorf("update failed: %w", err)
	}

	userUUID := s.parseUserUUID(postModel.UserId)
	translationService := NewTranslationService(s.db)
	if err := translationService.UpdateTranslation(ctx, existing.Id, defaultLocale, post.Title, post.Content, userUUID); err != nil {
		return "", fmt.Errorf("failed to update translation: %w", err)
	}

	metricsService := NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, existing.Id, int64(post.ViewsCount), int64(post.LikesCount), int64(post.CommentsCount)); err != nil {
		return "", fmt.Errorf("failed to update metrics: %w", err)
	}

	s.handleCommentImport(ctx, existing.Id, existing.Slug, post.Comments, opts, metricsService)

	return "updated", nil
}

func (s *ImporterService) createNewPost(ctx context.Context, postModel Post, post importer.Post, defaultLocale string, opts importer.ImportOptions) (string, error) {
	if err := s.crud.Create(ctx, postModel); err != nil {
		return "", fmt.Errorf("create failed: %w", err)
	}

	translations := map[string]*PostTranslationContent{
		defaultLocale: {
			Title:   post.Title,
			Content: post.Content,
		},
	}

	userUUID := s.parseUserUUID(postModel.UserId)
	translationService := NewTranslationService(s.db)
	if err := translationService.CreateTranslations(ctx, postModel.Id, translations, userUUID); err != nil {
		_ = s.crud.Delete(ctx, postModel.Id)
		return "", fmt.Errorf("failed to create translations: %w", err)
	}

	metricsService := NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, postModel.Id, int64(post.ViewsCount), int64(post.LikesCount), int64(post.CommentsCount)); err != nil {
		_ = s.crud.Delete(ctx, postModel.Id)
		return "", fmt.Errorf("failed to set metrics: %w", err)
	}

	s.handleCommentImport(ctx, postModel.Id, postModel.Slug, post.Comments, opts, metricsService)

	return "created", nil
}

func (s *ImporterService) parseUserUUID(userID *string) *uuid.UUID {
	if userID == nil {
		return nil
	}
	parsed, err := uuid.Parse(*userID)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *ImporterService) handleCommentImport(ctx context.Context, postID, slug string, comments []engines.Comment, opts importer.ImportOptions, metricsService *MetricsService) {
	if !opts.ImportComments || len(comments) == 0 {
		return
	}

	count, err := s.importComments(ctx, postID, comments, opts.UserID)
	if err != nil {
		if s.reporter != nil {
			s.reporter.Error(fmt.Errorf("failed to import comments for post %s: %w", slug, err))
		}
		return
	}

	if err := metricsService.SetMetric(ctx, postID, MetricNameComments, int64(count)); err != nil {
		if s.reporter != nil {
			s.reporter.Error(fmt.Errorf("failed to update comment count metric: %w", err))
		}
	}
}

func (s *ImporterService) postToModel(post importer.Post, userID string) Post {
	status := types.PostStatusDrafted
	var publishedAt *time.Time

	if post.PublishedAt != "" {
		if parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", post.PublishedAt); err == nil {
			status = types.PostStatusPublished
			publishedAt = &parsedTime
		}
	}

	slug := post.Slug
	if slug == "" {
		slug = slugify(post.Title)
	}

	postModel := Post{
		Id:          uuid.New().String(),
		Slug:        slug,
		Status:      status,
		PublishedAt: publishedAt,
		UserId:      &userID,
	}

	return postModel
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func (s *ImporterService) userExists(ctx context.Context, userID string) (bool, error) {
	sql, args, err := s.qb.
		Select("id").
		From("users").
		Where(query.Eq("id", userID)).
		Limit(1).
		Build()
	if err != nil {
		return false, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return rows.Next(), nil
}

func (s *ImporterService) truncatePosts(ctx context.Context) error {
	// Delete related data first (translations table uses polymorphic relationship)
	// Delete translations for posts
	deleteTranslations := "DELETE FROM translations WHERE translatable = 'post'"
	if _, err := s.db.Exec(ctx, deleteTranslations); err != nil {
		return fmt.Errorf("failed to delete post translations: %w", err)
	}

	// Delete posts (CASCADE will handle comments, likes, and other foreign key relations)
	var truncateSQL string

	switch s.db.DriverName() {
	case "postgres":
		// Use TRUNCATE CASCADE to automatically delete related records with foreign keys
		truncateSQL = "TRUNCATE TABLE post CASCADE"
	case "mysql":
		// MySQL doesn't support CASCADE with TRUNCATE, use DELETE
		truncateSQL = "DELETE FROM post"
	case "sqlite":
		// SQLite doesn't support TRUNCATE, use DELETE
		truncateSQL = "DELETE FROM post"
	default:
		truncateSQL = "DELETE FROM post"
	}

	if _, err := s.db.Exec(ctx, truncateSQL); err != nil {
		return fmt.Errorf("failed to truncate posts: %w", err)
	}

	return nil
}

func (s *ImporterService) findBySlug(ctx context.Context, slug string) (*Post, error) {
	sql, args, err := s.qb.
		Select("id", "user_id", "slug", "status", "published_at", "updated_at", "created_at").
		From("post").
		Where(query.Eq("slug", slug)).
		Limit(1).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	var post Post
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, nil
	}

	if err := rows.Scan(
		&post.Id,
		&post.UserId,
		&post.Slug,
		&post.Status,
		&post.PublishedAt,
		&post.UpdatedAt,
		&post.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return &post, nil
}

// importComments recursively imports comments and their children, returns count of imported comments
func (s *ImporterService) importComments(ctx context.Context, postID string, comments []engines.Comment, userID string) (int, error) {
	totalCount := 0
	for _, comment := range comments {
		count, err := s.importComment(ctx, postID, comment, nil, userID)
		if err != nil {
			return totalCount, fmt.Errorf("failed to import comment %s: %w", comment.ID, err)
		}
		totalCount += count
	}
	return totalCount, nil
}

// importComment imports a single comment and recursively imports its children, returns count of imported comments
func (s *ImporterService) importComment(ctx context.Context, postID string, comment engines.Comment, parentID *string, userID string) (int, error) {
	// Check if comment already exists (by checking if we have this comment ID already imported)
	// We'll use a simple check - if it fails to insert due to duplicate, skip it
	commentID := uuid.New().String()
	status := "published" // Default status for imported comments (valid statuses: awaiting, published, moderated, draft)

	var createdAt *time.Time
	if comment.CreatedAt != "" {
		if parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", comment.CreatedAt); err == nil {
			createdAt = &parsedTime
		}
	}

	// Insert comment into database
	var sql string
	var args []interface{}

	switch s.db.DriverName() {
	case "postgres":
		sql = `
			INSERT INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
		`
		args = []interface{}{commentID, userID, postID, "post", parentID, comment.Content, status, createdAt}
	case "mysql":
		sql = `
			INSERT IGNORE INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		args = []interface{}{commentID, userID, postID, "post", parentID, comment.Content, status, createdAt}
	case "sqlite":
		sql = `
			INSERT OR IGNORE INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		args = []interface{}{commentID, userID, postID, "post", parentID, comment.Content, status, createdAt}
	default:
		return 0, fmt.Errorf("unsupported database driver: %s", s.db.DriverName())
	}

	result, err := s.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to insert comment: %w", err)
	}

	// Count this comment if it was actually inserted
	count := 0
	if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
		count = 1
	}

	// Recursively import child comments
	if len(comment.Children) > 0 {
		for _, child := range comment.Children {
			childCount, err := s.importComment(ctx, postID, child, &commentID, userID)
			if err != nil {
				return count, fmt.Errorf("failed to import child comment: %w", err)
			}
			count += childCount
		}
	}

	return count, nil
}
