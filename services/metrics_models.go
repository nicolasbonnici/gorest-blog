package services

import "time"

// PostMetrics represents metrics data for a post
type PostMetrics struct {
	PostID    string     `json:"postId" db:"post_id"`
	Views     int64      `json:"views" db:"views"`
	Likes     int64      `json:"likes" db:"likes"`
	Comments  int64      `json:"comments" db:"comments"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt *time.Time `json:"createdAt,omitempty" db:"created_at"`
}

// MetricRecord represents a record from the metrics table
type MetricRecord struct {
	Resource   string `db:"resource"`
	ResourceID string `db:"resource_id"`
	Name       string `db:"name"`
	Value      int64  `db:"value"`
}
