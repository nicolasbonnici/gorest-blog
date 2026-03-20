package dtos

import (
	"time"

	"github.com/nicolasbonnici/gorest-blog/types"
)

type PostTranslationContentDTO struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type PostCreateDTO struct {
	Slug         string                                `json:"slug"`
	Status       types.PostStatus                      `json:"status"`
	Translations map[string]*PostTranslationContentDTO `json:"translations"`
}

type PostUpdateDTO struct {
	Slug         *string                               `json:"slug,omitempty"`
	Status       *types.PostStatus                     `json:"status,omitempty"`
	PublishedAt  *time.Time                            `json:"publishedAt,omitempty"`
	Translations map[string]*PostTranslationContentDTO `json:"translations,omitempty"`
}

type PostMetricsDTO struct {
	PostID    string     `json:"postId"`
	Views     int64      `json:"views"`
	Likes     int64      `json:"likes"`
	Comments  int64      `json:"comments"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

type PostResponseDTO struct {
	ID             string                                `json:"id"`
	UserID         *string                               `json:"userId,omitempty"`
	Slug           string                                `json:"slug"`
	Status         types.PostStatus                      `json:"status"`
	PublishedAt    *time.Time                            `json:"publishedAt,omitempty"`
	RemoteSourceID *string                               `json:"remoteSourceId,omitempty"`
	RemoteSource   *string                               `json:"remoteSource,omitempty"`
	UpdatedAt      *time.Time                            `json:"updatedAt,omitempty"`
	CreatedAt      *time.Time                            `json:"createdAt,omitempty"`
	Translations   map[string]*PostTranslationContentDTO `json:"translations,omitempty"`
	Metrics        *PostMetricsDTO                       `json:"metrics,omitempty"`
}
