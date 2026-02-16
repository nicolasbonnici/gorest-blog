package blog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"
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
func (s *TranslationService) CreateTranslations(ctx context.Context, postID string, translations map[string]*PostTranslationContent, userID *uuid.UUID) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	if len(translations) == 0 {
		return errors.New("at least one translation is required")
	}

	// Create all translations
	for locale, translationContent := range translations {
		if translationContent == nil {
			return fmt.Errorf("translation for locale %s cannot be nil", locale)
		}

		if err := translationContent.Validate(); err != nil {
			return fmt.Errorf("validation failed for locale %s: %w", locale, err)
		}

		translationContent.Sanitize()

		jsonContent, err := translationContent.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize content for locale %s: %w", locale, err)
		}

		translationID := uuid.New()
		now := time.Now()

		sql, args, err := s.qb.
			Insert("translations").
			Columns("id", "user_id", "translatable_id", "translatable", "locale", "content", "created_at").
			Values(translationID, userID, postUUID, TranslatableTypePost, locale, jsonContent, now).
			Build()
		if err != nil {
			return fmt.Errorf("failed to build insert query for locale %s: %w", locale, err)
		}

		_, err = s.db.Exec(ctx, sql, args...)
		if err != nil {
			return fmt.Errorf("failed to create translation for locale %s: %w", locale, err)
		}
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

	translationContent := &PostTranslationContent{
		Title:   title,
		Content: content,
	}

	if err := translationContent.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	translationContent.Sanitize()

	jsonContent, err := translationContent.ToJSON()
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
func (s *TranslationService) GetTranslation(ctx context.Context, postID, locale string) (*PostTranslationContent, error) {
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
func (s *TranslationService) ListTranslations(ctx context.Context, postID string) (map[string]*PostTranslationContent, error) {
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

	translations := make(map[string]*PostTranslationContent)

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

// UpdateTranslation updates an existing translation
func (s *TranslationService) UpdateTranslation(ctx context.Context, postID, locale, title, content string, userID *uuid.UUID) error {
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

	translationContent := &PostTranslationContent{
		Title:   title,
		Content: content,
	}

	if err := translationContent.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	translationContent.Sanitize()

	jsonContent, err := translationContent.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize content: %w", err)
	}

	now := time.Now()

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

// DeleteAllTranslations deletes all translations for a post
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

// DeleteTranslation deletes a translation
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

// postExists checks if a post exists in the database
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

// translationExists checks if a translation exists and optionally verifies ownership
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
	Posts []*Post
	Total *int
}

func (s *TranslationService) LoadPostsWithTranslations(ctx context.Context, limit, offset int, includeCount bool, conditions []query.Condition, orderBy []crud.OrderByClause) (*PostWithTranslationsResult, error) {
	sql, args, err := s.buildJoinQuery(limit, offset, conditions, orderBy)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	postsMap, postOrder, err := s.processRows(rows)
	if err != nil {
		return nil, err
	}

	posts := make([]*Post, 0, len(postOrder))
	for _, id := range postOrder {
		posts = append(posts, postsMap[id])
	}

	result := &PostWithTranslationsResult{
		Posts: posts,
		Total: nil,
	}

	if includeCount {
		total, err := s.getPostCount(ctx, conditions)
		if err != nil {
			return nil, err
		}
		result.Total = &total
	}

	return result, nil
}

type postRowData struct {
	id          string
	userID      *string
	slug        string
	status      string
	publishedAt interface{}
	updatedAt   interface{}
	createdAt   interface{}
	locale      *string
	content     *string
	metricName  *string
	metricValue *int64
}

func (s *TranslationService) processRows(rows database.Rows) (map[string]*Post, []string, error) {
	postsMap := make(map[string]*Post)
	var postOrder []string

	for rows.Next() {
		rowData, err := s.scanRow(rows)
		if err != nil {
			return nil, nil, err
		}

		post := s.ensurePostExists(postsMap, &postOrder, rowData)

		if err := s.addTranslationToPost(post, rowData); err != nil {
			return nil, nil, err
		}

		s.addMetricToPost(post, rowData)
	}

	return postsMap, postOrder, nil
}

func (s *TranslationService) scanRow(rows database.Rows) (*postRowData, error) {
	var rowData postRowData
	err := rows.Scan(
		&rowData.id,
		&rowData.userID,
		&rowData.slug,
		&rowData.status,
		&rowData.publishedAt,
		&rowData.updatedAt,
		&rowData.createdAt,
		&rowData.locale,
		&rowData.content,
		&rowData.metricName,
		&rowData.metricValue,
	)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	return &rowData, nil
}

func (s *TranslationService) ensurePostExists(postsMap map[string]*Post, postOrder *[]string, rowData *postRowData) *Post {
	post, exists := postsMap[rowData.id]
	if exists {
		return post
	}

	post = &Post{
		Id:           rowData.id,
		UserId:       rowData.userID,
		Slug:         rowData.slug,
		Status:       types.PostStatus(rowData.status),
		PublishedAt:  s.parseTime(rowData.publishedAt),
		UpdatedAt:    s.parseTime(rowData.updatedAt),
		CreatedAt:    s.parseTime(rowData.createdAt),
		Translations: make(map[string]*PostTranslationContent),
		Metrics:      &PostMetrics{PostID: rowData.id, Views: 0, Likes: 0, Comments: 0},
	}
	postsMap[rowData.id] = post
	*postOrder = append(*postOrder, rowData.id)
	return post
}

func (s *TranslationService) parseTime(val interface{}) *time.Time {
	if val == nil {
		return nil
	}
	t, _ := parseTimeValue(val)
	return t
}

func (s *TranslationService) addTranslationToPost(post *Post, rowData *postRowData) error {
	if rowData.locale == nil || rowData.content == nil {
		return nil
	}

	translationContent, err := ParsePostTranslationContent(*rowData.content)
	if err != nil {
		return fmt.Errorf("failed to parse translation for post %s, locale %s: %w", rowData.id, *rowData.locale, err)
	}
	post.Translations[*rowData.locale] = translationContent
	return nil
}

func (s *TranslationService) addMetricToPost(post *Post, rowData *postRowData) {
	if rowData.metricName == nil || rowData.metricValue == nil || post.Metrics == nil {
		return
	}

	switch *rowData.metricName {
	case MetricNameViews:
		post.Metrics.Views = *rowData.metricValue
	case MetricNameLikes:
		post.Metrics.Likes = *rowData.metricValue
	case MetricNameComments:
		post.Metrics.Comments = *rowData.metricValue
	}
}

func (s *TranslationService) getPostCount(ctx context.Context, conditions []query.Condition) (int, error) {
	countSQL, countArgs, err := s.buildCountQuery(conditions)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	var total int
	if err := s.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to get total count: %w", err)
	}
	return total, nil
}

// buildJoinQuery constructs a SQL query with JOIN to fetch posts, translations, and metrics
func (s *TranslationService) buildJoinQuery(limit, offset int, conditions []query.Condition, orderBy []crud.OrderByClause) (string, []interface{}, error) {
	dialect := s.db.Dialect()
	args := []interface{}{}
	argPos := 1

	// Build WHERE clause for subquery
	whereClauses := []string{}
	for _, condition := range conditions {
		condSQL, condArgs, nextParam := condition.ToSQL(dialect, argPos)
		whereClauses = append(whereClauses, condSQL)
		args = append(args, condArgs...)
		argPos = nextParam
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				whereClause += " AND "
			}
			whereClause += clause
		}
	}

	// Build ORDER BY clause
	orderClause := ""
	if len(orderBy) > 0 {
		orderClause = " ORDER BY "
		for i, order := range orderBy {
			if i > 0 {
				orderClause += ", "
			}
			orderClause += order.Column + " " + order.Direction.String()
		}
	} else {
		orderClause = " ORDER BY p.created_at DESC"
	}

	// Use subquery to limit distinct posts first, then join translations and metrics
	baseSQL := `SELECT
		p.id, p.user_id, p.slug, p.status, p.published_at, p.updated_at, p.created_at,
		t.locale, t.content,
		m.name, m.value
	FROM (
		SELECT id, user_id, slug, status, published_at, updated_at, created_at
		FROM post p` + whereClause + orderClause + ` ` + dialect.LimitOffset(limit, offset) + `
	) p
	LEFT JOIN translations t ON t.translatable_id = p.id AND t.translatable = ` + dialect.Placeholder(argPos)

	args = append(args, TranslatableTypePost)
	argPos++

	baseSQL += `
	LEFT JOIN metrics m ON m.resource_id = p.id AND m.resource = ` + dialect.Placeholder(argPos)

	args = append(args, MetricResourcePost)

	baseSQL += orderClause

	return baseSQL, args, nil
}

// buildCountQuery constructs a SQL query to count distinct posts
func (s *TranslationService) buildCountQuery(conditions []query.Condition) (string, []interface{}, error) {
	dialect := s.db.Dialect()
	args := []interface{}{}
	argPos := 1

	baseSQL := "SELECT COUNT(DISTINCT p.id) FROM post p"

	whereClauses := []string{}
	for _, condition := range conditions {
		condSQL, condArgs, nextParam := condition.ToSQL(dialect, argPos)
		whereClauses = append(whereClauses, condSQL)
		args = append(args, condArgs...)
		argPos = nextParam
	}

	if len(whereClauses) > 0 {
		baseSQL += " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				baseSQL += " AND "
			}
			baseSQL += clause
		}
	}

	return baseSQL, args, nil
}

// parseTimeValue parses a time value from different database types
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
