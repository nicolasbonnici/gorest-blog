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
	engine, ok := engines.Get(opts.Source)
	if !ok {
		return nil, fmt.Errorf("unknown engine: %s (available: %v)", opts.Source, engines.List())
	}

	if opts.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	userExists, err := s.userExists(ctx, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate user: %w", err)
	}
	if !userExists {
		return nil, fmt.Errorf("user_id '%s' does not exist", opts.UserID)
	}

	var posts []importer.Post

	if opts.Username != "" {
		posts, err = engine.FetchByUsername(ctx, opts.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts by username: %w", err)
		}
	} else if opts.ArticleURL != "" {
		post, err := engine.FetchByURL(ctx, opts.ArticleURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by URL: %w", err)
		}
		posts = []importer.Post{*post}
	} else if opts.ArticleID != "" {
		post, err := engine.FetchByID(ctx, opts.ArticleID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by ID: %w", err)
		}
		posts = []importer.Post{*post}
	} else {
		return nil, fmt.Errorf("one of username, url, or id must be provided")
	}

	result := &importer.ImportResult{
		TotalFetched: len(posts),
		Errors:       make([]error, 0),
	}

	// Truncate existing posts if requested
	if opts.Truncate && !opts.DryRun {
		if err := s.truncatePosts(ctx); err != nil {
			return nil, fmt.Errorf("failed to truncate posts: %w", err)
		}
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

		switch action {
		case "created":
			result.Created++
		case "updated":
			result.Updated++
		case "skipped":
			result.Skipped++
		}

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

func (s *ImporterService) importPost(ctx context.Context, post importer.Post, opts importer.ImportOptions) (string, error) {
	postModel := s.postToModel(post, opts.UserID)

	// Default locale for imported content (could be configurable)
	defaultLocale := "en"
	translations := map[string]*PostTranslationContent{
		defaultLocale: {
			Title:   post.Title,
			Content: post.Content,
		},
	}

	if opts.DryRun {
		existing, err := s.findBySlug(ctx, postModel.Slug)
		if err == nil && existing != nil {
			return "updated", nil
		}
		return "created", nil
	}

	existing, err := s.findBySlug(ctx, postModel.Slug)
	if err == nil && existing != nil {
		// Always update existing post (upsert behavior)
		if err := s.crud.Update(ctx, existing.Id, postModel); err != nil {
			return "", fmt.Errorf("update failed: %w", err)
		}

		// Update translation for default locale
		translationService := NewTranslationService(s.db)
		var userUUID *uuid.UUID
		if postModel.UserId != nil {
			parsed, err := uuid.Parse(*postModel.UserId)
			if err == nil {
				userUUID = &parsed
			}
		}
		if err := translationService.UpdateTranslation(ctx, existing.Id, defaultLocale, post.Title, post.Content, userUUID); err != nil {
			return "", fmt.Errorf("failed to update translation: %w", err)
		}

		return "updated", nil
	}

	// Create new post
	if err := s.crud.Create(ctx, postModel); err != nil {
		return "", fmt.Errorf("create failed: %w", err)
	}

	// Create translations
	translationService := NewTranslationService(s.db)
	var userUUID *uuid.UUID
	if postModel.UserId != nil {
		parsed, err := uuid.Parse(*postModel.UserId)
		if err == nil {
			userUUID = &parsed
		}
	}
	if err := translationService.CreateTranslations(ctx, postModel.Id, translations, userUUID); err != nil {
		// Rollback: delete the post if translation creation fails
		_ = s.crud.Delete(ctx, postModel.Id)
		return "", fmt.Errorf("failed to create translations: %w", err)
	}

	return "created", nil
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
