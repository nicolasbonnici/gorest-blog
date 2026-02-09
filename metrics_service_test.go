package blog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
)

func setupMetricsTestDB(t *testing.T) database.Database {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}

	ctx := context.Background()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS metrics (
			resource TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			name TEXT NOT NULL,
			value INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (resource, resource_id, name)
		)
	`
	if _, err := db.Exec(ctx, createTableSQL); err != nil {
		t.Fatalf("Failed to create metrics table: %v", err)
	}

	return db
}

func TestMetricsService_InitializeMetrics(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.PostID != postID {
		t.Errorf("Expected PostID %s, got %s", postID, metrics.PostID)
	}

	if metrics.Views != 0 {
		t.Errorf("Expected Views to be 0, got %d", metrics.Views)
	}

	if metrics.Likes != 0 {
		t.Errorf("Expected Likes to be 0, got %d", metrics.Likes)
	}

	if metrics.Comments != 0 {
		t.Errorf("Expected Comments to be 0, got %d", metrics.Comments)
	}
}

func TestMetricsService_IncrementViews(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementViews(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment views: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Views != 1 {
		t.Errorf("Expected Views to be 1, got %d", metrics.Views)
	}

	err = service.IncrementViews(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment views again: %v", err)
	}

	metrics, err = service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Views != 2 {
		t.Errorf("Expected Views to be 2, got %d", metrics.Views)
	}
}

func TestMetricsService_IncrementLikes(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment likes: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Likes != 1 {
		t.Errorf("Expected Likes to be 1, got %d", metrics.Likes)
	}
}

func TestMetricsService_DecrementLikes(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment likes: %v", err)
	}

	err = service.IncrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment likes again: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Likes != 2 {
		t.Errorf("Expected Likes to be 2, got %d", metrics.Likes)
	}

	err = service.DecrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to decrement likes: %v", err)
	}

	metrics, err = service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Likes != 1 {
		t.Errorf("Expected Likes to be 1 after decrement, got %d", metrics.Likes)
	}
}

func TestMetricsService_DecrementLikesMinimumZero(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.DecrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to decrement likes: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Likes != 0 {
		t.Errorf("Expected Likes to remain at 0, got %d", metrics.Likes)
	}
}

func TestMetricsService_IncrementComments(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Comments != 1 {
		t.Errorf("Expected Comments to be 1, got %d", metrics.Comments)
	}
}

func TestMetricsService_DecrementComments(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments again: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Comments != 2 {
		t.Errorf("Expected Comments to be 2, got %d", metrics.Comments)
	}

	err = service.DecrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to decrement comments: %v", err)
	}

	metrics, err = service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Comments != 1 {
		t.Errorf("Expected Comments to be 1 after decrement, got %d", metrics.Comments)
	}
}

func TestMetricsService_GetMetricsNonExistent(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Views != 0 || metrics.Likes != 0 || metrics.Comments != 0 {
		t.Errorf("Expected all metrics to be 0 for non-existent post, got Views=%d, Likes=%d, Comments=%d",
			metrics.Views, metrics.Likes, metrics.Comments)
	}
}

func TestMetricsService_InvalidPostID(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	invalidID := "not-a-uuid"

	err := service.InitializeMetrics(ctx, invalidID)
	if err == nil {
		t.Error("Expected error for invalid post ID, got nil")
	}

	err = service.IncrementViews(ctx, invalidID)
	if err == nil {
		t.Error("Expected error for invalid post ID in IncrementViews, got nil")
	}

	_, err = service.GetMetrics(ctx, invalidID)
	if err == nil {
		t.Error("Expected error for invalid post ID in GetMetrics, got nil")
	}
}

func TestMetricsService_MultipleMetrics(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)
	ctx := context.Background()

	postID := uuid.New().String()

	err := service.InitializeMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	err = service.IncrementViews(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment views: %v", err)
	}

	err = service.IncrementViews(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment views: %v", err)
	}

	err = service.IncrementLikes(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment likes: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments: %v", err)
	}

	err = service.IncrementComments(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to increment comments: %v", err)
	}

	metrics, err := service.GetMetrics(ctx, postID)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.Views != 2 {
		t.Errorf("Expected Views to be 2, got %d", metrics.Views)
	}

	if metrics.Likes != 1 {
		t.Errorf("Expected Likes to be 1, got %d", metrics.Likes)
	}

	if metrics.Comments != 3 {
		t.Errorf("Expected Comments to be 3, got %d", metrics.Comments)
	}
}
