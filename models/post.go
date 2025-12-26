package models

import "time"

type Post struct {
	Id          string     `json:"id,omitempty" db:"id"`
	UserId      *string    `json:"userId,omitempty" db:"user_id"`
	Slug        string     `json:"slug" db:"slug"`
	Status      string     `json:"status" db:"status"`
	Title       string     `json:"title" db:"title"`
	Content     string     `json:"content" db:"content"`
	PublishedAt *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt   *time.Time `json:"createdAt,omitempty" db:"created_at"`
}

func (Post) TableName() string {
	return "post"
}
