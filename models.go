package blog

import (
	"errors"
	"html"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog/types"
)

type Post struct {
	Id          string           `json:"id,omitempty" db:"id"`
	UserId      *string          `json:"userId,omitempty" db:"user_id"`
	Slug        string           `json:"slug" db:"slug"`
	Status      types.PostStatus `json:"status" db:"status"`
	Title       string           `json:"title" db:"title"`
	Content     string           `json:"content" db:"content"`
	PublishedAt *time.Time       `json:"publishedAt,omitempty" db:"published_at"`
	UpdatedAt   *time.Time       `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt   *time.Time       `json:"createdAt,omitempty" db:"created_at"`
}

func (Post) TableName() string {
	return "post"
}

type CreatePostRequest struct {
	Slug    string           `json:"slug" validate:"required"`
	Status  types.PostStatus `json:"status" validate:"required"`
	Title   string           `json:"title" validate:"required"`
	Content string           `json:"content" validate:"required"`
}

type UpdatePostRequest struct {
	Slug    *string           `json:"slug,omitempty"`
	Status  *types.PostStatus `json:"status,omitempty"`
	Title   *string           `json:"title,omitempty"`
	Content *string           `json:"content,omitempty"`
}

func (r *CreatePostRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.New("title cannot be empty")
	}

	r.Slug = strings.TrimSpace(r.Slug)
	if r.Slug == "" {
		return errors.New("slug cannot be empty")
	}

	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.New("content cannot be empty")
	}

	if !r.Status.IsValid() {
		return errors.New("invalid status value")
	}

	r.Content = html.EscapeString(r.Content)

	return nil
}

func (r *UpdatePostRequest) Validate() error {
	if r.Title != nil {
		*r.Title = strings.TrimSpace(*r.Title)
		if *r.Title == "" {
			return errors.New("title cannot be empty")
		}
	}

	if r.Slug != nil {
		*r.Slug = strings.TrimSpace(*r.Slug)
		if *r.Slug == "" {
			return errors.New("slug cannot be empty")
		}
	}

	if r.Content != nil {
		*r.Content = strings.TrimSpace(*r.Content)
		if *r.Content == "" {
			return errors.New("content cannot be empty")
		}
		*r.Content = html.EscapeString(*r.Content)
	}

	if r.Status != nil && !r.Status.IsValid() {
		return errors.New("invalid status value")
	}

	return nil
}
