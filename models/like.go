package models

import "time"

type Like struct {
	Id         string     `json:"id,omitempty" db:"id"`
	LikerId    *string    `json:"likerId,omitempty" db:"liker_id"`
	LikedId    *string    `json:"likedId,omitempty" db:"liked_id"`
	Likeable   string     `json:"likeable" db:"likeable"`
	LikeableId string     `json:"likeableId" db:"likeable_id"`
	LikedAt    time.Time  `json:"likedAt" db:"liked_at"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt  *time.Time `json:"createdAt,omitempty" db:"created_at"`
}

func (Like) TableName() string {
	return "likes"
}
