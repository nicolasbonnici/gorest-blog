package models

import "time"

type Comment struct {
	Id        string     `json:"id,omitempty" db:"id"`
	UserId    *string    `json:"userId,omitempty" db:"user_id"`
	PostId    *string    `json:"postId,omitempty" db:"post_id"`
	ParentId  *string    `json:"parentId,omitempty" db:"parent_id"`
	Content   string     `json:"content" db:"content"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt *time.Time `json:"createdAt,omitempty" db:"created_at"`
}

func (Comment) TableName() string {
	return "comment"
}
