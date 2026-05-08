package blog

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"

	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest-blog/importer/engines"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/services"
	"github.com/nicolasbonnici/gorest-blog/types"
)

type ImporterService struct {
	crud     *crud.CRUD[models.Post]
	db       database.Database
	qb       *query.Builder
	reporter importer.ProgressReporter
}

func NewImporterService(db database.Database, reporter importer.ProgressReporter) *ImporterService {
	if reporter == nil {
		reporter = &importer.NoOpProgressReporter{}
	}
	return &ImporterService{
		crud:     crud.New[models.Post](db),
		db:       db,
		qb:       query.New(db.Dialect()),
		reporter: reporter,
	}
}

func (s *ImporterService) Import(ctx context.Context, opts importer.ImportOptions) (*importer.ImportResult, error) {
	if !opts.SyncMode.IsValid() {
		opts.SyncMode = importer.SyncModeLocalWins
	}

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

	if opts.SyncMode == importer.SyncModeLocalWins {
		syncResult, err := s.Sync(ctx, opts, engine, posts)
		if err != nil {
			return nil, err
		}
		return s.syncResultToImportResult(syncResult), nil
	}

	return s.processPosts(ctx, posts, opts)
}

func (s *ImporterService) syncResultToImportResult(syncResult *importer.SyncResult) *importer.ImportResult {
	importResult := &importer.ImportResult{
		TotalFetched: syncResult.LocalCreated + syncResult.LocalUpdated + syncResult.Skipped,
		Created:      syncResult.LocalCreated,
		Updated:      syncResult.LocalUpdated,
		Skipped:      syncResult.Skipped,
		Failed:       len(syncResult.Errors),
		Errors:       make([]error, 0, len(syncResult.Errors)),
	}
	for _, syncErr := range syncResult.Errors {
		importResult.Errors = append(importResult.Errors, syncErr.Error)
	}
	return importResult
}

func (s *ImporterService) Sync(ctx context.Context, opts importer.ImportOptions, engine engines.Engine, remotePosts []importer.Post) (*importer.SyncResult, error) {
	remoteMap := make(map[string]importer.Post)
	for _, post := range remotePosts {
		remoteMap[post.Slug] = post
	}

	localPosts, err := s.fetchLocalPosts(ctx, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch local posts: %w", err)
	}

	localMap := make(map[string]*models.Post)
	for i := range localPosts {
		localMap[localPosts[i].Slug] = &localPosts[i]
	}

	switch opts.SyncMode {
	case importer.SyncModeLocalWins:
		return s.syncLocalWins(ctx, localMap, remoteMap, engine, opts), nil
	case importer.SyncModeRemoteWins:
		return s.syncRemoteWins(ctx, localMap, remoteMap, opts), nil
	case importer.SyncModeImportOnly:
		return s.syncImportOnly(ctx, localMap, remoteMap, opts), nil
	default:
		return nil, fmt.Errorf("invalid sync mode: %s", opts.SyncMode)
	}
}

func (s *ImporterService) syncLocalWins(ctx context.Context, localMap map[string]*models.Post, remoteMap map[string]importer.Post, engine engines.Engine, opts importer.ImportOptions) *importer.SyncResult {
	result := &importer.SyncResult{
		Errors: make([]importer.SyncError, 0),
	}

	totalOps := len(remoteMap) + len(localMap)
	if s.reporter != nil {
		s.reporter.Start(totalOps, fmt.Sprintf("Syncing posts (bidirectional) from %s", opts.Source))
	}

	current := 0

	current = s.syncRemoteToLocal(ctx, remoteMap, localMap, opts, result, current)
	s.syncLocalToRemote(ctx, localMap, remoteMap, engine, opts, result, &current)

	if s.reporter != nil {
		s.reporter.Finish(result.String())
	}

	return result
}

func (s *ImporterService) syncRemoteToLocal(ctx context.Context, remoteMap map[string]importer.Post, localMap map[string]*models.Post, opts importer.ImportOptions, result *importer.SyncResult, current int) int {
	for slug, remotePost := range remoteMap {
		current++
		if s.reporter != nil {
			s.reporter.Update(current, fmt.Sprintf("Importing: %s", remotePost.Title))
		}

		if _, existsLocal := localMap[slug]; !existsLocal {
			if err := s.importRemotePost(ctx, remotePost, opts); err != nil {
				result.Errors = append(result.Errors, importer.SyncError{
					PostSlug:  slug,
					Operation: "import",
					Error:     err,
				})
			} else {
				result.LocalCreated++
			}
		}
	}
	return current
}

func (s *ImporterService) syncLocalToRemote(ctx context.Context, localMap map[string]*models.Post, remoteMap map[string]importer.Post, engine engines.Engine, opts importer.ImportOptions, result *importer.SyncResult, current *int) {
	for slug, localPost := range localMap {
		*current++
		if s.reporter != nil {
			s.reporter.Update(*current, fmt.Sprintf("Syncing: %s", slug))
		}

		remotePost, existsRemote := remoteMap[slug]

		if !existsRemote {
			s.createRemotePost(ctx, engine, localPost, slug, opts, result)
		} else {
			s.updateRemotePost(ctx, engine, localPost, remotePost, slug, opts, result)
		}
	}
}

func (s *ImporterService) createRemotePost(ctx context.Context, engine engines.Engine, localPost *models.Post, slug string, opts importer.ImportOptions, result *importer.SyncResult) {
	if opts.DryRun {
		result.RemoteCreated++
		return
	}

	if err := s.pushPostToRemote(ctx, engine, localPost, opts, true); err != nil {
		result.Errors = append(result.Errors, importer.SyncError{
			PostSlug:  slug,
			Operation: "export-create",
			Error:     err,
		})
	} else {
		result.RemoteCreated++
	}
}

func (s *ImporterService) updateRemotePost(ctx context.Context, engine engines.Engine, localPost *models.Post, remotePost importer.Post, slug string, opts importer.ImportOptions, result *importer.SyncResult) {
	needsUpdate, err := s.hasLocalChanges(ctx, localPost, remotePost)
	if err != nil {
		result.Errors = append(result.Errors, importer.SyncError{
			PostSlug:  slug,
			Operation: "check-changes",
			Error:     err,
		})
		return
	}

	if !needsUpdate && !opts.ForceUpdate {
		result.Skipped++
		return
	}

	if opts.DryRun {
		result.RemoteUpdated++
		return
	}

	if err := s.pushPostToRemote(ctx, engine, localPost, opts, false); err != nil {
		result.Errors = append(result.Errors, importer.SyncError{
			PostSlug:  slug,
			Operation: "export-update",
			Error:     err,
		})
	} else {
		result.RemoteUpdated++
	}
}

func (s *ImporterService) syncRemoteWins(ctx context.Context, localMap map[string]*models.Post, remoteMap map[string]importer.Post, opts importer.ImportOptions) *importer.SyncResult {
	result := &importer.SyncResult{
		Errors: make([]importer.SyncError, 0),
	}

	if s.reporter != nil {
		s.reporter.Start(len(remoteMap), fmt.Sprintf("Importing from %s (remote wins)", opts.Source))
	}

	current := 0
	for slug, remotePost := range remoteMap {
		current++
		if s.reporter != nil {
			s.reporter.Update(current, fmt.Sprintf("Processing: %s", remotePost.Title))
		}

		if localPost, existsLocal := localMap[slug]; existsLocal {
			if err := s.updateFromRemote(ctx, localPost, remotePost, opts); err != nil {
				result.Errors = append(result.Errors, importer.SyncError{
					PostSlug:  slug,
					Operation: "update",
					Error:     err,
				})
			} else {
				result.LocalUpdated++
			}
		} else {
			if err := s.importRemotePost(ctx, remotePost, opts); err != nil {
				result.Errors = append(result.Errors, importer.SyncError{
					PostSlug:  slug,
					Operation: "import",
					Error:     err,
				})
			} else {
				result.LocalCreated++
			}
		}
	}

	if s.reporter != nil {
		s.reporter.Finish(result.String())
	}

	return result
}

func (s *ImporterService) syncImportOnly(ctx context.Context, localMap map[string]*models.Post, remoteMap map[string]importer.Post, opts importer.ImportOptions) *importer.SyncResult {
	result := &importer.SyncResult{
		Errors: make([]importer.SyncError, 0),
	}

	if s.reporter != nil {
		s.reporter.Start(len(remoteMap), fmt.Sprintf("Importing new posts from %s", opts.Source))
	}

	current := 0
	for slug, remotePost := range remoteMap {
		current++
		if s.reporter != nil {
			s.reporter.Update(current, fmt.Sprintf("Checking: %s", remotePost.Title))
		}

		if localPost, existsLocal := localMap[slug]; !existsLocal {
			if err := s.importRemotePost(ctx, remotePost, opts); err != nil {
				result.Errors = append(result.Errors, importer.SyncError{
					PostSlug:  slug,
					Operation: "import",
					Error:     err,
				})
			} else {
				result.LocalCreated++
			}
		} else if opts.ForceUpdate {
			if err := s.updateFromRemote(ctx, localPost, remotePost, opts); err != nil {
				result.Errors = append(result.Errors, importer.SyncError{
					PostSlug:  slug,
					Operation: "update",
					Error:     err,
				})
			} else {
				result.LocalUpdated++
			}
		} else {
			result.Skipped++
		}
	}

	if s.reporter != nil {
		s.reporter.Finish(result.String())
	}

	return result
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

		action, commentsCount, err := s.importPost(ctx, post, opts)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to import '%s': %w", post.Title, err))
			if s.reporter != nil {
				s.reporter.Error(err)
			}
			continue
		}

		s.updateResultCounts(result, action)
		result.CommentsCreated += commentsCount

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

func (s *ImporterService) importPost(ctx context.Context, post importer.Post, opts importer.ImportOptions) (string, int, error) {
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

func (s *ImporterService) handleDryRun(ctx context.Context, slug string) (string, int, error) {
	existing, err := s.findBySlug(ctx, slug)
	if err == nil && existing != nil {
		return "updated", 0, nil
	}
	return "created", 0, nil
}

func (s *ImporterService) updateExistingPost(ctx context.Context, existing *models.Post, postModel models.Post, post importer.Post, defaultLocale string, opts importer.ImportOptions) (string, int, error) {
	if err := s.crud.Update(ctx, existing.ID, postModel); err != nil {
		return "", 0, fmt.Errorf("update failed: %w", err)
	}

	userUUID := s.parseUserUUID(postModel.UserID)
	translationService := services.NewTranslationService(s.db)
	if err := translationService.UpdateTranslation(ctx, existing.ID, defaultLocale, post.Title, post.Content, userUUID); err != nil {
		return "", 0, fmt.Errorf("failed to update translation: %w", err)
	}

	metricsService := services.NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, existing.ID, int64(post.ViewsCount), int64(post.LikesCount), int64(post.CommentsCount)); err != nil {
		return "", 0, fmt.Errorf("failed to update metrics: %w", err)
	}

	commentsCount := s.handleCommentImport(ctx, existing.ID, existing.Slug, post.Comments, opts, metricsService)

	return "updated", commentsCount, nil
}

func (s *ImporterService) createNewPost(ctx context.Context, postModel models.Post, post importer.Post, defaultLocale string, opts importer.ImportOptions) (string, int, error) {
	if err := s.crud.Create(ctx, postModel); err != nil {
		return "", 0, fmt.Errorf("create failed: %w", err)
	}

	translations := map[string]*models.PostTranslationContent{
		defaultLocale: {
			Title:   post.Title,
			Content: post.Content,
		},
	}

	userUUID := s.parseUserUUID(postModel.UserID)
	translationService := services.NewTranslationService(s.db)
	if err := translationService.CreateTranslations(ctx, postModel.ID, translations, userUUID); err != nil {
		_ = s.crud.Delete(ctx, postModel.ID)
		return "", 0, fmt.Errorf("failed to create translations: %w", err)
	}

	metricsService := services.NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, postModel.ID, int64(post.ViewsCount), int64(post.LikesCount), int64(post.CommentsCount)); err != nil {
		_ = s.crud.Delete(ctx, postModel.ID)
		return "", 0, fmt.Errorf("failed to set metrics: %w", err)
	}

	commentsCount := s.handleCommentImport(ctx, postModel.ID, postModel.Slug, post.Comments, opts, metricsService)

	return "created", commentsCount, nil
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

func (s *ImporterService) handleCommentImport(ctx context.Context, postID, slug string, comments []engines.Comment, opts importer.ImportOptions, metricsService *services.MetricsService) int {
	if !opts.ImportComments || len(comments) == 0 {
		return 0
	}

	count, err := s.importComments(ctx, postID, comments, opts.UserID)
	if err != nil {
		if s.reporter != nil {
			s.reporter.Error(fmt.Errorf("failed to import comments for post %s: %w", slug, err))
		}
		return 0
	}

	if err := metricsService.SetMetric(ctx, postID, services.MetricNameComments, int64(count)); err != nil {
		if s.reporter != nil {
			s.reporter.Error(fmt.Errorf("failed to update comment count metric: %w", err))
		}
	}

	return count
}

func (s *ImporterService) postToModel(post importer.Post, userID string) models.Post {
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

	postModel := models.Post{
		ID:          uuid.New().String(),
		Slug:        slug,
		Status:      status,
		PublishedAt: publishedAt,
		UserID:      &userID,
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

func (s *ImporterService) findBySlug(ctx context.Context, slug string) (*models.Post, error) {
	sql, args, err := s.qb.
		Select("id", "user_id", "slug", "status", "published_at", "remote_source_id", "remote_source", "updated_at", "created_at").
		From("post").
		Where(query.Eq("slug", slug)).
		Limit(1).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	var post models.Post
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, nil
	}

	if err := rows.Scan(
		&post.ID,
		&post.UserID,
		&post.Slug,
		&post.Status,
		&post.PublishedAt,
		&post.RemoteSourceID,
		&post.RemoteSource,
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

func (s *ImporterService) importComment(ctx context.Context, postID string, comment engines.Comment, parentID *string, userID string) (int, error) {
	if existingID := s.checkExistingComment(ctx, comment, postID, userID); existingID != nil {
		return s.importChildComments(ctx, postID, comment.Children, existingID, userID)
	}

	commentID := uuid.New().String()
	status := "published"
	createdAt := s.parseCommentCreatedAt(comment.CreatedAt)

	count, err := s.insertComment(ctx, commentID, userID, postID, parentID, comment, status, createdAt)
	if err != nil {
		return 0, err
	}

	childCount, err := s.importChildComments(ctx, postID, comment.Children, &commentID, userID)
	if err != nil {
		return count, err
	}

	return count + childCount, nil
}

func (s *ImporterService) checkExistingComment(ctx context.Context, comment engines.Comment, postID, userID string) *string {
	if comment.ID == "" {
		return nil
	}

	existingID, err := s.findCommentByRemoteSource(ctx, comment.ID, "devto")
	if err == nil && existingID != nil {
		return existingID
	}
	return nil
}

func (s *ImporterService) importChildComments(ctx context.Context, postID string, children []engines.Comment, parentID *string, userID string) (int, error) {
	count := 0
	for _, child := range children {
		childCount, err := s.importComment(ctx, postID, child, parentID, userID)
		if err != nil {
			return count, fmt.Errorf("failed to import child comment: %w", err)
		}
		count += childCount
	}
	return count, nil
}

func (s *ImporterService) parseCommentCreatedAt(createdAtStr string) *time.Time {
	if createdAtStr == "" {
		return nil
	}

	parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", createdAtStr)
	if err != nil {
		return nil
	}
	return &parsedTime
}

func (s *ImporterService) insertComment(ctx context.Context, commentID, userID, postID string, parentID *string, comment engines.Comment, status string, createdAt *time.Time) (int, error) {
	sql, args := s.buildCommentInsertSQL(commentID, userID, postID, parentID, comment, status, createdAt)

	result, err := s.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to insert comment: %w", err)
	}

	if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
		return 1, nil
	}
	return 0, nil
}

func (s *ImporterService) buildCommentInsertSQL(commentID, userID, postID string, parentID *string, comment engines.Comment, status string, createdAt *time.Time) (string, []interface{}) {
	args := []interface{}{commentID, userID, postID, "post", parentID, comment.Content, status, comment.ID, "devto", createdAt}

	switch s.db.DriverName() {
	case "postgres":
		sql := `
			INSERT INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, remote_source_id, remote_source, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (remote_source_id, remote_source) DO NOTHING
		`
		return sql, args
	case "mysql":
		sql := `
			INSERT IGNORE INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, remote_source_id, remote_source, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		return sql, args
	default: // sqlite or default
		sql := `
			INSERT OR IGNORE INTO comment (id, user_id, commentable_id, commentable, parent_id, content, status, remote_source_id, remote_source, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		return sql, args
	}
}

func (s *ImporterService) importRemotePost(ctx context.Context, post importer.Post, opts importer.ImportOptions) error {
	postModel := s.postToModel(post, opts.UserID)
	defaultLocale := "en"

	if opts.DryRun {
		return nil
	}

	if err := s.crud.Create(ctx, postModel); err != nil {
		return fmt.Errorf("create failed: %w", err)
	}

	postModel.RemoteSourceID = &post.ID
	source := opts.Source
	postModel.RemoteSource = &source

	if err := s.crud.Update(ctx, postModel.ID, postModel); err != nil {
		_ = s.crud.Delete(ctx, postModel.ID)
		return fmt.Errorf("failed to update remote source ID: %w", err)
	}

	translations := map[string]*models.PostTranslationContent{
		defaultLocale: {
			Title:   post.Title,
			Content: post.Content,
		},
	}

	userUUID := s.parseUserUUID(postModel.UserID)
	translationService := services.NewTranslationService(s.db)
	if err := translationService.CreateTranslations(ctx, postModel.ID, translations, userUUID); err != nil {
		_ = s.crud.Delete(ctx, postModel.ID)
		return fmt.Errorf("failed to create translations: %w", err)
	}

	metricsService := services.NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, postModel.ID, int64(post.ViewsCount), int64(post.LikesCount), int64(post.CommentsCount)); err != nil {
		_ = s.crud.Delete(ctx, postModel.ID)
		return fmt.Errorf("failed to set metrics: %w", err)
	}

	s.handleCommentImport(ctx, postModel.ID, postModel.Slug, post.Comments, opts, metricsService)

	return nil
}

func (s *ImporterService) pushPostToRemote(ctx context.Context, engine engines.Engine, localPost *models.Post, opts importer.ImportOptions, isCreate bool) error {
	if opts.APIKey == "" {
		return fmt.Errorf("API key required for pushing changes (use --api-key or DEVTO_API_KEY)")
	}

	translationService := services.NewTranslationService(s.db)
	translations, err := translationService.ListTranslations(ctx, localPost.ID)
	if err != nil {
		return fmt.Errorf("failed to load translations: %w", err)
	}

	defaultTranslation := translations["en"]
	if defaultTranslation == nil {
		for _, trans := range translations {
			defaultTranslation = trans
			break
		}
	}
	if defaultTranslation == nil {
		return fmt.Errorf("no translations found for post")
	}

	var publishedAt string
	if localPost.Status == types.PostStatusPublished && localPost.PublishedAt != nil {
		publishedAt = localPost.PublishedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	exportPost := importer.Post{
		Slug:        localPost.Slug,
		Title:       defaultTranslation.Title,
		Content:     defaultTranslation.Content,
		PublishedAt: publishedAt,
	}

	if isCreate {
		remoteID, err := engine.CreatePost(ctx, opts.APIKey, exportPost)
		if err != nil {
			return fmt.Errorf("failed to create remote post: %w", err)
		}

		if err := s.storeRemoteID(ctx, localPost.ID, opts.Source, remoteID); err != nil {
			return fmt.Errorf("failed to store remote ID: %w", err)
		}
	} else {
		remoteID, err := s.getRemoteID(ctx, localPost.ID, opts.Source)
		if err != nil {
			return fmt.Errorf("failed to get remote ID: %w", err)
		}

		if err := engine.UpdatePost(ctx, opts.APIKey, remoteID, exportPost); err != nil {
			return fmt.Errorf("failed to update remote post: %w", err)
		}
	}

	return nil
}

func (s *ImporterService) hasLocalChanges(ctx context.Context, local *models.Post, remote importer.Post) (bool, error) {
	translationService := services.NewTranslationService(s.db)
	translations, err := translationService.ListTranslations(ctx, local.ID)
	if err != nil {
		return false, fmt.Errorf("failed to load translations: %w", err)
	}

	localTrans := translations["en"]
	if localTrans == nil {
		for _, trans := range translations {
			localTrans = trans
			break
		}
	}
	if localTrans == nil {
		return false, nil
	}

	if localTrans.Title != remote.Title {
		return true, nil
	}
	if localTrans.Content != remote.Content {
		return true, nil
	}

	localPublished := local.Status == types.PostStatusPublished
	remotePublished := remote.PublishedAt != ""
	if localPublished != remotePublished {
		return true, nil
	}

	return false, nil
}

func (s *ImporterService) updateFromRemote(ctx context.Context, existing *models.Post, remotePost importer.Post, opts importer.ImportOptions) error {
	postModel := s.postToModel(remotePost, opts.UserID)
	postModel.ID = existing.ID

	if opts.DryRun {
		return nil
	}

	if err := s.crud.Update(ctx, existing.ID, postModel); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	userUUID := s.parseUserUUID(postModel.UserID)
	translationService := services.NewTranslationService(s.db)
	if err := translationService.UpdateTranslation(ctx, existing.ID, "en", remotePost.Title, remotePost.Content, userUUID); err != nil {
		return fmt.Errorf("failed to update translation: %w", err)
	}

	metricsService := services.NewMetricsService(s.db)
	if err := metricsService.SetMetrics(ctx, existing.ID, int64(remotePost.ViewsCount), int64(remotePost.LikesCount), int64(remotePost.CommentsCount)); err != nil {
		return fmt.Errorf("failed to update metrics: %w", err)
	}

	s.handleCommentImport(ctx, existing.ID, existing.Slug, remotePost.Comments, opts, metricsService)

	return nil
}

func (s *ImporterService) fetchLocalPosts(ctx context.Context, userID string) ([]models.Post, error) {
	sql, args, err := s.qb.
		Select("id", "user_id", "slug", "status", "published_at", "remote_source_id", "remote_source", "updated_at", "created_at").
		From("post").
		Where(query.Eq("user_id", userID)).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		if err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Slug,
			&post.Status,
			&post.PublishedAt,
			&post.RemoteSourceID,
			&post.RemoteSource,
			&post.UpdatedAt,
			&post.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		posts = append(posts, post)
	}

	return posts, nil
}

func (s *ImporterService) storeRemoteID(ctx context.Context, postID, source, remoteID string) error {
	sql, args, err := s.qb.
		Update("post").
		Set("remote_source_id", remoteID).
		Set("remote_source", source).
		Where(query.Eq("id", postID)).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	if _, err := s.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("failed to store remote ID: %w", err)
	}

	return nil
}

func (s *ImporterService) getRemoteID(ctx context.Context, postID, source string) (string, error) {
	sql, args, err := s.qb.
		Select("remote_source_id").
		From("post").
		Where(query.Eq("id", postID)).
		Where(query.Eq("remote_source", source)).
		Limit(1).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return "", fmt.Errorf("remote ID not found for post %s", postID)
	}

	var remoteID *string
	if err := rows.Scan(&remoteID); err != nil {
		return "", fmt.Errorf("scan failed: %w", err)
	}

	if remoteID == nil || *remoteID == "" {
		return "", fmt.Errorf("remote ID is empty for post %s", postID)
	}

	return *remoteID, nil
}

func (s *ImporterService) findCommentByRemoteSource(ctx context.Context, remoteSourceID, remoteSource string) (*string, error) {
	sql, args, err := s.qb.
		Select("id").
		From("comment").
		Where(query.Eq("remote_source_id", remoteSourceID)).
		Where(query.Eq("remote_source", remoteSource)).
		Limit(1).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, nil // Not found
	}

	var commentID string
	if err := rows.Scan(&commentID); err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return &commentID, nil
}
