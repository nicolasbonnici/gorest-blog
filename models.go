package blog

import (
	"errors"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog/types"
)

type Post struct {
	Id           string                             `json:"id,omitempty" db:"id" rbac:"read:*;write:none"`
	UserId       *string                            `json:"userId,omitempty" db:"user_id" rbac:"read:*;write:writer"`
	Slug         string                             `json:"slug" db:"slug" rbac:"read:*;write:writer"`
	Status       types.PostStatus                   `json:"status" db:"status" rbac:"read:*;write:moderator,writer"`
	PublishedAt  *time.Time                         `json:"publishedAt,omitempty" db:"published_at" rbac:"read:*;write:writer"`
	UpdatedAt    *time.Time                         `json:"updatedAt,omitempty" db:"updated_at" rbac:"read:*;write:none"`
	CreatedAt    *time.Time                         `json:"createdAt,omitempty" db:"created_at" rbac:"read:*;write:none"`
	Translations map[string]*PostTranslationContent `json:"translations" db:"-" rbac:"read:*;write:writer"`
	Metrics      *PostMetrics                       `json:"metrics,omitempty" db:"-" rbac:"read:*;write:none"`
}

func (Post) TableName() string {
	return "post"
}

type CreatePostRequest struct {
	Slug         string                             `json:"slug" validate:"required"`
	Status       types.PostStatus                   `json:"status" validate:"required"`
	Translations map[string]*PostTranslationContent `json:"translations" validate:"required"`
}

type UpdatePostRequest struct {
	Slug         string                             `json:"slug"`
	Status       types.PostStatus                   `json:"status"`
	PublishedAt  *time.Time                         `json:"publishedAt,omitempty"`
	Translations map[string]*PostTranslationContent `json:"translations"`
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
	// Slug is optional for updates, but if provided must not be empty
	if r.Slug != "" {
		r.Slug = strings.TrimSpace(r.Slug)
		if r.Slug == "" {
			return errors.New("slug cannot be empty")
		}
	}

	// Status is optional for updates, but if provided must be valid
	if r.Status != "" {
		if !r.Status.IsValid() {
			return errors.New("invalid status value")
		}
	}

	// Translations are optional for updates, but if provided must be valid
	if len(r.Translations) > 0 {
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
	}

	return nil
}
