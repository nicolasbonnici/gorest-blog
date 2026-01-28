package blog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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
			Insert("translatable").
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
		Insert("translatable").
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
		From("translatable").
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
		From("translatable").
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
		Update("translatable").
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
		Delete("translatable").
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
		Delete("translatable").
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
		From("translatable").
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
