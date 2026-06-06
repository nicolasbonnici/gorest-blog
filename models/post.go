package models

import (
	"errors"
	"html"
	"strings"
	"time"

	taxonomy "github.com/nicolasbonnici/gorest-taxonomy"

	"github.com/nicolasbonnici/gorest-blog/types"
)

type Post struct {
	ID             string                             `json:"id,omitempty" db:"id"`
	UserID         *string                            `json:"userId,omitempty" db:"user_id"`
	Slug           string                             `json:"slug" db:"slug"`
	Status         types.PostStatus                   `json:"status" db:"status"`
	PublishedAt    *time.Time                         `json:"publishedAt,omitempty" db:"published_at"`
	RemoteSourceID *string                            `json:"remoteSourceId,omitempty" db:"remote_source_id"`
	RemoteSource   *string                            `json:"remoteSource,omitempty" db:"remote_source"`
	UpdatedAt      *time.Time                         `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt      *time.Time                         `json:"createdAt,omitempty" db:"created_at"`
	Translations   map[string]*PostTranslationContent `json:"translations,omitempty" db:"-"`
	Metrics        *PostMetrics                       `json:"metrics,omitempty" db:"-"`
	Categories     []taxonomy.Category                `json:"categories,omitempty" db:"-"`
	Tags           []taxonomy.Tag                     `json:"tags,omitempty" db:"-"`
}

func (Post) TableName() string {
	return "post"
}

type PostTranslationContent struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (p *PostTranslationContent) Validate() error {
	p.Title = strings.TrimSpace(p.Title)
	p.Content = strings.TrimSpace(p.Content)

	if p.Title == "" {
		return errors.New("title cannot be empty")
	}

	if p.Content == "" {
		return errors.New("content cannot be empty")
	}

	return nil
}

func (p *PostTranslationContent) Sanitize() {
	p.Title = html.EscapeString(p.Title)
	// Content is markdown and should NOT be HTML-escaped as it breaks code blocks
	// and markdown syntax. Markdown renderers handle XSS protection when converting to HTML.
}

type PostMetrics struct {
	PostID    string     `json:"postId" db:"post_id"`
	Views     int64      `json:"views" db:"views"`
	Likes     int64      `json:"likes" db:"likes"`
	Comments  int64      `json:"comments" db:"comments"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt *time.Time `json:"createdAt,omitempty" db:"created_at"`
}
