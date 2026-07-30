package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"

	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/types"
)

const (
	// TranslatableTypePost is the type identifier for post translations
	TranslatableTypePost = "post"
)

// TranslationService provides methods to manage post translations
type TranslationService struct {
	db database.Database
	qb *query.Builder
}

// NewTranslationService creates a new TranslationService instance
func NewTranslationService(db database.Database) *TranslationService {
	return &TranslationService{
		db: db,
		qb: query.New(db.Dialect()),
	}
}

// CreateTranslations creates multiple translations for a post in a single operation
func (s *TranslationService) CreateTranslations(ctx context.Context, postID string, translations map[string]*models.PostTranslationContent, userID *uuid.UUID) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	if len(translations) == 0 {
		return errors.New("at least one translation is required")
	}

	now := time.Now()
	insert := s.qb.
		Insert("translations").
		Columns("id", "user_id", "translatable_id", "translatable", "locale", "content", "created_at")

	// Accumulate every locale into one multi-row INSERT so a post with N
	// translations costs a single round-trip instead of N.
	for locale, translationContent := range translations {
		if translationContent == nil {
			return fmt.Errorf("translation for locale %s cannot be nil", locale)
		}

		if err := translationContent.Validate(); err != nil {
			return fmt.Errorf("validation failed for locale %s: %w", locale, err)
		}

		translationContent.Sanitize()

		jsonContent, err := ToJSON(translationContent)
		if err != nil {
			return fmt.Errorf("failed to serialize content for locale %s: %w", locale, err)
		}

		insert.Values(uuid.New(), userID, postUUID, TranslatableTypePost, locale, jsonContent, now)
	}

	sql, args, err := insert.Build()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	if _, err := s.db.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("failed to create translations: %w", err)
	}

	return nil
}

// CreateTranslation creates a new translation for a post
func (s *TranslationService) CreateTranslation(ctx context.Context, postID, locale, title, content string, userID *uuid.UUID) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	exists, err := s.postExists(ctx, postUUID)
	if err != nil {
		return fmt.Errorf("failed to validate post: %w", err)
	}
	if !exists {
		return errors.New("post not found")
	}

	translationContent := &models.PostTranslationContent{
		Title:   title,
		Content: content,
	}

	if err := translationContent.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	translationContent.Sanitize()

	jsonContent, err := ToJSON(translationContent)
	if err != nil {
		return fmt.Errorf("failed to serialize content: %w", err)
	}

	translationID := uuid.New()
	now := time.Now()

	sql, args, err := s.qb.
		Insert("translations").
		Columns("id", "user_id", "translatable_id", "translatable", "locale", "content", "created_at").
		Values(translationID, userID, postUUID, TranslatableTypePost, locale, jsonContent, now).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to create translation: %w", err)
	}

	return nil
}

// GetTranslation retrieves a specific translation for a post
func (s *TranslationService) GetTranslation(ctx context.Context, postID, locale string) (*models.PostTranslationContent, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	sql, args, err := s.qb.
		Select("content").
		From("translations").
		Where(
			query.And(
				query.Eq("translatable_id", postUUID),
				query.Eq("translatable", TranslatableTypePost),
				query.Eq("locale", locale),
			),
		).
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
		return nil, errors.New("translation not found")
	}

	var content string
	if err := rows.Scan(&content); err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return ParsePostTranslationContent(content)
}

// ListTranslations retrieves all translations for a post
func (s *TranslationService) ListTranslations(ctx context.Context, postID string) (map[string]*models.PostTranslationContent, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	sql, args, err := s.qb.
		Select("locale", "content").
		From("translations").
		Where(
			query.And(
				query.Eq("translatable_id", postUUID),
				query.Eq("translatable", TranslatableTypePost),
			),
		).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	translations := make(map[string]*models.PostTranslationContent)

	for rows.Next() {
		var locale, content string
		if err := rows.Scan(&locale, &content); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		translationContent, err := ParsePostTranslationContent(content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse content for locale %s: %w", locale, err)
		}

		translations[locale] = translationContent
	}

	return translations, nil
}

func (s *TranslationService) UpdateTranslation(ctx context.Context, postID, locale, title, content string, userID *uuid.UUID) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	translationContent := &models.PostTranslationContent{
		Title:   title,
		Content: content,
	}

	if err := translationContent.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	translationContent.Sanitize()

	jsonContent, err := ToJSON(translationContent)
	if err != nil {
		return fmt.Errorf("failed to serialize content: %w", err)
	}

	exists, err := s.translationExists(ctx, postUUID, locale, nil)
	if err != nil {
		return fmt.Errorf("failed to verify translation: %w", err)
	}

	now := time.Now()

	if exists {
		sql, args, err := s.qb.
			Update("translations").
			Set("content", jsonContent).
			Set("updated_at", now).
			Where(
				query.And(
					query.Eq("translatable_id", postUUID),
					query.Eq("translatable", TranslatableTypePost),
					query.Eq("locale", locale),
				),
			).
			Build()
		if err != nil {
			return fmt.Errorf("failed to build update query: %w", err)
		}

		_, err = s.db.Exec(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("failed to update translation: %w", err)
		}

		return nil
	}

	translationID := uuid.New()

	sql, args, err := s.qb.
		Insert("translations").
		Columns("id", "user_id", "translatable_id", "translatable", "locale", "content", "created_at").
		Values(translationID, userID, postUUID, TranslatableTypePost, locale, jsonContent, now).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to create translation: %w", err)
	}

	return nil
}

func (s *TranslationService) DeleteAllTranslations(ctx context.Context, postID string) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	sql, args, err := s.qb.
		Delete("translations").
		Where(
			query.And(
				query.Eq("translatable_id", postUUID),
				query.Eq("translatable", TranslatableTypePost),
			),
		).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to delete translations: %w", err)
	}

	return nil
}

func (s *TranslationService) DeleteTranslation(ctx context.Context, postID, locale string, userID *uuid.UUID) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	exists, err := s.translationExists(ctx, postUUID, locale, userID)
	if err != nil {
		return fmt.Errorf("failed to verify translation: %w", err)
	}
	if !exists {
		return errors.New("translation not found or access denied")
	}

	sql, args, err := s.qb.
		Delete("translations").
		Where(
			query.And(
				query.Eq("translatable_id", postUUID),
				query.Eq("translatable", TranslatableTypePost),
				query.Eq("locale", locale),
			),
		).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to delete translation: %w", err)
	}

	return nil
}

func (s *TranslationService) postExists(ctx context.Context, postID uuid.UUID) (bool, error) {
	sql, args, err := s.qb.
		Select("id").
		From("post").
		Where(query.Eq("id", postID)).
		Limit(1).
		Build()
	if err != nil {
		return false, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("failed to check post existence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return rows.Next(), nil
}

func (s *TranslationService) translationExists(ctx context.Context, postID uuid.UUID, locale string, userID *uuid.UUID) (bool, error) {
	conditions := []query.Condition{
		query.Eq("translatable_id", postID),
		query.Eq("translatable", TranslatableTypePost),
		query.Eq("locale", locale),
	}

	if userID != nil {
		conditions = append(conditions, query.Eq("user_id", *userID))
	}

	sql, args, err := s.qb.
		Select("id").
		From("translations").
		Where(query.And(conditions...)).
		Limit(1).
		Build()
	if err != nil {
		return false, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("failed to check translation existence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return rows.Next(), nil
}

type PostWithTranslationsResult struct {
	Posts []*models.Post
	Total *int
}

func (s *TranslationService) LoadPostsWithTranslations(ctx context.Context, limit, offset int, includeCount bool, conditions []query.Condition, orderBy []crud.OrderByClause, titleSearch string, countMode crud.CountMode) (*PostWithTranslationsResult, error) {
	posts, byID, err := s.loadPosts(ctx, limit, offset, conditions, orderBy, titleSearch)
	if err != nil {
		return nil, err
	}

	// Translations and metrics are loaded side by side rather than joined onto
	// the posts query: joining both multiplies rows (a post with 11 locales and
	// 5 metrics arrives 55 times), and the fan-out grows with every new locale.
	if len(posts) > 0 {
		ids := make([]any, 0, len(posts))
		for _, post := range posts {
			ids = append(ids, post.ID)
		}

		if err := s.attachTranslations(ctx, byID, ids); err != nil {
			return nil, err
		}
		if err := s.attachMetrics(ctx, byID, ids); err != nil {
			return nil, err
		}
	}

	result := &PostWithTranslationsResult{
		Posts: posts,
		Total: nil,
	}

	if includeCount {
		total, err := s.countPosts(ctx, countMode, limit, offset, len(posts), conditions, titleSearch)
		if err != nil {
			return nil, err
		}
		result.Total = total
	}

	return result, nil
}

// loadPosts fetches one page of post metadata, returning them in query order
// alongside an index by id for the attach* loaders to fill in.
func (s *TranslationService) loadPosts(ctx context.Context, limit, offset int, conditions []query.Condition, orderBy []crud.OrderByClause, titleSearch string) ([]*models.Post, map[string]*models.Post, error) {
	sb := s.qb.Select("p.id", "p.user_id", "p.slug", "p.status", "p.visual", "p.published_at", "p.updated_at", "p.created_at").
		From("post").As("p")

	for _, cond := range conditions {
		sb = sb.Where(cond)
	}
	if titleSearch != "" {
		sb = sb.Where(s.buildTitleSearchCondition(titleSearch))
	}

	if len(orderBy) > 0 {
		for _, order := range orderBy {
			sb = sb.OrderBy(order.Column, order.Direction)
		}
	} else {
		sb = sb.OrderBy("p.created_at", query.DESC)
	}

	sql, args, err := sb.Limit(limit).Offset(offset).Build()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	posts := make([]*models.Post, 0, limit)
	byID := make(map[string]*models.Post, limit)

	for rows.Next() {
		var (
			id, slug, status              string
			userID, visual                *string
			publishedAt, updatedAt, added any
		)
		if err := rows.Scan(&id, &userID, &slug, &status, &visual, &publishedAt, &updatedAt, &added); err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}

		post := &models.Post{
			ID:           id,
			UserID:       userID,
			Slug:         slug,
			Status:       types.PostStatus(status),
			Visual:       visual,
			PublishedAt:  s.parseTime(publishedAt),
			UpdatedAt:    s.parseTime(updatedAt),
			CreatedAt:    s.parseTime(added),
			Translations: make(map[string]*models.PostTranslationContent),
			Metrics:      &models.PostMetrics{PostID: id},
		}
		posts = append(posts, post)
		byID[id] = post
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return posts, byID, nil
}

func (s *TranslationService) attachTranslations(ctx context.Context, byID map[string]*models.Post, ids []any) error {
	sql, args, err := s.qb.
		Select("t.translatable_id", "t.locale", "t.content").
		From("translations").As("t").
		Where(query.Eq("t.translatable", TranslatableTypePost)).
		Where(query.In("t.translatable_id", ids...)).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build translations query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("translations query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var postID string
		var locale, content *string
		if err := rows.Scan(&postID, &locale, &content); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		post, ok := byID[postID]
		if !ok || locale == nil || content == nil {
			continue
		}

		translationContent, err := ParsePostTranslationContent(*content)
		if err != nil {
			return fmt.Errorf("failed to parse translation for post %s, locale %s: %w", postID, *locale, err)
		}
		post.Translations[*locale] = translationContent
	}

	return rows.Err()
}

func (s *TranslationService) attachMetrics(ctx context.Context, byID map[string]*models.Post, ids []any) error {
	sql, args, err := s.qb.
		Select("m.resource_id", "m.name", "m.value").
		From("metrics").As("m").
		Where(query.Eq("m.resource", MetricResourcePost)).
		Where(query.In("m.resource_id", ids...)).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build metrics query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("metrics query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var postID string
		var name *string
		var value *int64
		if err := rows.Scan(&postID, &name, &value); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		post, ok := byID[postID]
		if !ok || name == nil || value == nil || post.Metrics == nil {
			continue
		}

		switch *name {
		case MetricNameViews:
			post.Metrics.Views = *value
		case MetricNameLikes:
			post.Metrics.Likes = *value
		case MetricNameComments:
			post.Metrics.Comments = *value
		}
	}

	return rows.Err()
}

// countPosts applies the same rules as crud.GetAllPaginated: a page shorter than
// the limit already reveals the total, an estimate answers unfiltered listings,
// and anything else counts exactly. The inner CTE limits posts rather than
// joined rows, so len(posts) is what the limit was applied to.
func (s *TranslationService) countPosts(ctx context.Context, mode crud.CountMode, limit, offset, returned int, conditions []query.Condition, titleSearch string) (*int, error) {
	if mode == crud.CountNone {
		return nil, nil
	}

	if limit <= 0 || returned < limit {
		total := offset + returned
		return &total, nil
	}

	if mode == crud.CountEstimate && len(conditions) == 0 && titleSearch == "" {
		if total, ok := s.estimatePostCount(ctx); ok {
			return total, nil
		}
	}

	total, err := s.getPostCount(ctx, conditions, titleSearch)
	if err != nil {
		return nil, err
	}
	return &total, nil
}

// estimatePostCount reads the planner's row estimate. ok is false whenever the
// figure cannot be trusted, leaving the caller to count exactly.
func (s *TranslationService) estimatePostCount(ctx context.Context) (*int, bool) {
	estimator, ok := s.db.Dialect().(database.RowEstimator)
	if !ok {
		return nil, false
	}

	estimateQuery, args, ok := estimator.EstimateRowsQuery("post")
	if !ok {
		return nil, false
	}

	var estimate int64
	if err := s.db.QueryRow(ctx, estimateQuery, args...).Scan(&estimate); err != nil {
		return nil, false
	}

	if estimate < 0 {
		return nil, false
	}

	total := int(estimate)
	return &total, true
}

func (s *TranslationService) parseTime(val interface{}) *time.Time {
	if val == nil {
		return nil
	}
	t, _ := parseTimeValue(val)
	return t
}

func (s *TranslationService) getPostCount(ctx context.Context, conditions []query.Condition, titleSearch string) (int, error) {
	countSQL, countArgs, err := s.buildCountQuery(conditions, titleSearch)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	var total int
	if err := s.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to get total count: %w", err)
	}
	return total, nil
}

func (s *TranslationService) buildCountQuery(conditions []query.Condition, titleSearch string) (string, []interface{}, error) {
	sb := s.qb.Select().
		SelectExpr(query.CountDistinct(query.RawExpr("p.id"))).
		From("post").As("p")
	for _, cond := range conditions {
		sb = sb.Where(cond)
	}
	if titleSearch != "" {
		sb = sb.Where(s.buildTitleSearchCondition(titleSearch))
	}
	return sb.Build()
}

// buildTitleSearchCondition returns a condition that checks whether any translation
// of a post contains the given term in its title field.
// Uses native JSON extraction per dialect to avoid JSONB key-ordering issues.
func (s *TranslationService) buildTitleSearchCondition(term string) query.Condition {
	escaped := escapeLikePattern(term)
	pattern := "%" + escaped + "%"

	var titleCond query.Condition
	switch s.db.DriverName() {
	case "postgres":
		// JSONB ->> extracts as text; ILIKE is case-insensitive
		titleCond = query.Raw("content->>'title' ILIKE ?", pattern)
	case "mysql":
		titleCond = query.Raw("JSON_UNQUOTE(JSON_EXTRACT(content, '$.title')) LIKE ?", pattern)
	default: // sqlite
		titleCond = query.Raw("json_extract(content, '$.title') LIKE ?", pattern)
	}

	titleSubquery := s.qb.
		Select("translatable_id").
		From("translations").
		Where(query.And(
			query.Eq("translatable", TranslatableTypePost),
			titleCond,
		))
	return query.InSubquery("id", titleSubquery)
}

// escapeLikePattern escapes LIKE wildcard characters in user input.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func parseTimeValue(val interface{}) (*time.Time, error) {
	if val == nil {
		return nil, nil
	}

	switch v := val.(type) {
	case time.Time:
		return &v, nil
	case *time.Time:
		return v, nil
	case string:
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.999999999",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return &t, nil
			}
		}
		return nil, fmt.Errorf("unable to parse time string: %s", v)
	case []byte:
		return parseTimeValue(string(v))
	default:
		return nil, fmt.Errorf("unsupported time type: %T", val)
	}
}
