package blog

import (
	"errors"
	"html"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog/types"
)

type Post struct {
	Id           string                              `json:"id,omitempty" db:"id"`
	UserId       *string                             `json:"userId,omitempty" db:"user_id"`
	Slug         string                              `json:"slug" db:"slug"`
	Status       types.PostStatus                    `json:"status" db:"status"`
	PublishedAt  *time.Time                          `json:"publishedAt,omitempty" db:"published_at"`
	UpdatedAt    *time.Time                          `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt    *time.Time                          `json:"createdAt,omitempty" db:"created_at"`
	Translations map[string]*PostTranslationContent  `json:"translations" db:"-"`
}

func (Post) TableName() string {
	return "post"
}

type CreatePostRequest struct {
	Slug         string                                      `json:"slug" validate:"required"`
	Status       types.PostStatus                            `json:"status" validate:"required"`
	Translations map[string]*PostTranslationContent          `json:"translations" validate:"required"`
}

type UpdatePostRequest struct {
	Locale  string `json:"locale" validate:"required"`
	Title   string `json:"title" validate:"required"`
	Content string `json:"content" validate:"required"`
}

func (r *CreatePostRequest) Validate() error {
	r.Slug = strings.TrimSpace(r.Slug)
	if r.Slug == "" {
		return errors.New("slug cannot be empty")
	}

	if !r.Status.IsValid() {
		return errors.New("invalid status value")
	}

	if len(r.Translations) == 0 {
		return errors.New("at least one translation is required")
	}

	// Validate each translation
	for locale, translation := range r.Translations {
		if translation == nil {
			return errors.New("translation for locale " + locale + " cannot be nil")
		}

		if err := translation.Validate(); err != nil {
			return errors.New("validation failed for locale " + locale + ": " + err.Error())
		}

		// Apply HTML escaping to both title and content
		translation.Sanitize()
	}

	return nil
}

func (r *UpdatePostRequest) Validate() error {
	r.Locale = strings.TrimSpace(r.Locale)
	if r.Locale == "" {
		return errors.New("locale cannot be empty")
	}

	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.New("title cannot be empty")
	}

	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.New("content cannot be empty")
	}

	// Apply HTML escaping to both title and content
	r.Title = html.EscapeString(r.Title)
	r.Content = html.EscapeString(r.Content)

	return nil
}
