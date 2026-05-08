package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"

	"github.com/nicolasbonnici/gorest-blog/models"
)

const (
	MetricResourcePost = "post"
	MetricNameViews    = "views"
	MetricNameLikes    = "likes"
	MetricNameComments = "comments"
)

type MetricsService struct {
	db database.Database
	qb *query.Builder
}

func NewMetricsService(db database.Database) *MetricsService {
	return &MetricsService{
		db: db,
		qb: query.New(db.Dialect()),
	}
}

func (s *MetricsService) IncrementViews(ctx context.Context, postID string) error {
	return s.incrementMetric(ctx, postID, MetricNameViews)
}

func (s *MetricsService) IncrementLikes(ctx context.Context, postID string) error {
	return s.incrementMetric(ctx, postID, MetricNameLikes)
}

func (s *MetricsService) IncrementComments(ctx context.Context, postID string) error {
	return s.incrementMetric(ctx, postID, MetricNameComments)
}

func (s *MetricsService) DecrementLikes(ctx context.Context, postID string) error {
	return s.decrementMetric(ctx, postID, MetricNameLikes)
}

func (s *MetricsService) DecrementComments(ctx context.Context, postID string) error {
	return s.decrementMetric(ctx, postID, MetricNameComments)
}

func (s *MetricsService) GetMetrics(ctx context.Context, postID string) (*models.PostMetrics, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	sql, args, err := s.qb.
		Select("name", "value").
		From("metrics").
		Where(
			query.And(
				query.Eq("resource", MetricResourcePost),
				query.Eq("resource_id", postUUID),
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

	metrics := &models.PostMetrics{
		PostID:   postID,
		Views:    0,
		Likes:    0,
		Comments: 0,
	}

	for rows.Next() {
		var name string
		var value int64
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		switch name {
		case MetricNameViews:
			metrics.Views = value
		case MetricNameLikes:
			metrics.Likes = value
		case MetricNameComments:
			metrics.Comments = value
		}
	}

	return metrics, nil
}

func (s *MetricsService) InitializeMetrics(ctx context.Context, postID string) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	metricNames := []string{MetricNameViews, MetricNameLikes, MetricNameComments}

	for _, metricName := range metricNames {
		if err := s.initializeMetric(ctx, postUUID, metricName); err != nil {
			return fmt.Errorf("failed to initialize %s metric: %w", metricName, err)
		}
	}

	return nil
}

// SetMetric sets a metric to a specific value (used for importing)
func (s *MetricsService) SetMetric(ctx context.Context, postID, metricName string, value int64) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	var sql string
	var args []interface{}

	switch s.db.DriverName() {
	case "postgres":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = $4
		`
		args = []interface{}{MetricResourcePost, postUUID, metricName, value}
	case "mysql":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE value = ?
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName, value, value}
	case "sqlite":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = ?
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName, value, value}
	default:
		return fmt.Errorf("unsupported database driver: %s", s.db.DriverName())
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to set %s metric: %w", metricName, err)
	}

	return nil
}

// SetMetrics sets multiple metrics at once (convenience method for importing)
func (s *MetricsService) SetMetrics(ctx context.Context, postID string, views, likes, comments int64) error {
	if err := s.SetMetric(ctx, postID, MetricNameViews, views); err != nil {
		return err
	}
	if err := s.SetMetric(ctx, postID, MetricNameLikes, likes); err != nil {
		return err
	}
	if err := s.SetMetric(ctx, postID, MetricNameComments, comments); err != nil {
		return err
	}
	return nil
}

func (s *MetricsService) incrementMetric(ctx context.Context, postID, metricName string) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	var sql string
	var args []interface{}

	switch s.db.DriverName() {
	case "postgres":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES ($1, $2, $3, 1)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = metrics.value + 1
		`
		args = []interface{}{MetricResourcePost, postUUID, metricName}
	case "mysql":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 1)
			ON DUPLICATE KEY UPDATE value = value + 1
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	case "sqlite":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 1)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = value + 1
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	default:
		return fmt.Errorf("unsupported database driver: %s", s.db.DriverName())
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to increment %s: %w", metricName, err)
	}

	return nil
}

func (s *MetricsService) decrementMetric(ctx context.Context, postID, metricName string) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	var sql string
	var args []interface{}

	switch s.db.DriverName() {
	case "postgres":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES ($1, $2, $3, 0)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = GREATEST(metrics.value - 1, 0)
		`
		args = []interface{}{MetricResourcePost, postUUID, metricName}
	case "mysql":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 0)
			ON DUPLICATE KEY UPDATE value = GREATEST(value - 1, 0)
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	case "sqlite":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 0)
			ON CONFLICT (resource, resource_id, name)
			DO UPDATE SET value = MAX(value - 1, 0)
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	default:
		return fmt.Errorf("unsupported database driver: %s", s.db.DriverName())
	}

	_, err = s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to decrement %s: %w", metricName, err)
	}

	return nil
}

func (s *MetricsService) initializeMetric(ctx context.Context, postUUID uuid.UUID, metricName string) error {
	var sql string
	var args []interface{}

	switch s.db.DriverName() {
	case "postgres":
		sql = `
			INSERT INTO metrics (resource, resource_id, name, value)
			VALUES ($1, $2, $3, 0)
			ON CONFLICT (resource, resource_id, name) DO NOTHING
		`
		args = []interface{}{MetricResourcePost, postUUID, metricName}
	case "mysql":
		sql = `
			INSERT IGNORE INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 0)
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	case "sqlite":
		sql = `
			INSERT OR IGNORE INTO metrics (resource, resource_id, name, value)
			VALUES (?, ?, ?, 0)
		`
		args = []interface{}{MetricResourcePost, postUUID.String(), metricName}
	default:
		return fmt.Errorf("unsupported database driver: %s", s.db.DriverName())
	}

	_, err := s.db.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("failed to initialize metric: %w", err)
	}

	return nil
}
