package importer

import (
	"context"
	"fmt"

	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
)

type Repository interface {
	Create(ctx context.Context, post *models.Post) error
	Update(ctx context.Context, id string, post *models.Post) error
	FindByTitle(ctx context.Context, title string) (*models.Post, error)
	FindByID(ctx context.Context, id string) (*models.Post, error)
	UserExists(ctx context.Context, userID string) (bool, error)
}

type PostgresRepository struct {
	crud *crud.CRUD[models.Post]
	db   database.Database
}

func NewRepository(db database.Database) Repository {
	return &PostgresRepository{
		crud: crud.New[models.Post](db),
		db:   db,
	}
}

func (r *PostgresRepository) Create(ctx context.Context, post *models.Post) error {
	// Use explicit SQL to ensure published_at is properly handled
	query := `
		INSERT INTO post (user_id, slug, status, title, content, published_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		RETURNING id, created_at`

	rows, err := r.db.Query(ctx, query,
		post.UserId,
		post.Slug,
		post.Status,
		post.Title,
		post.Content,
		post.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&post.Id, &post.CreatedAt); err != nil {
			return fmt.Errorf("failed to scan created post: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) Update(ctx context.Context, id string, post *models.Post) error {
	// Use explicit SQL to ensure published_at is properly handled
	query := `
		UPDATE post
		SET user_id = $1, slug = $2, status = $3, title = $4, content = $5,
		    published_at = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`

	_, err := r.db.Query(ctx, query,
		post.UserId,
		post.Slug,
		post.Status,
		post.Title,
		post.Content,
		post.PublishedAt,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	return nil
}

func (r *PostgresRepository) FindByTitle(ctx context.Context, title string) (*models.Post, error) {
	query := "SELECT id, user_id, slug, status, title, content, published_at, updated_at, created_at FROM post WHERE title = $1 LIMIT 1"
	var post models.Post

	rows, err := r.db.Query(ctx, query, title)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	if err := rows.Scan(
		&post.Id,
		&post.UserId,
		&post.Slug,
		&post.Status,
		&post.Title,
		&post.Content,
		&post.PublishedAt,
		&post.UpdatedAt,
		&post.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return &post, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*models.Post, error) {
	post, err := r.crud.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find post: %w", err)
	}
	return post, nil
}

func (r *PostgresRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)"
	var exists bool

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return false, fmt.Errorf("no result from user existence check")
	}

	if err := rows.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to scan user existence result: %w", err)
	}

	return exists, nil
}
