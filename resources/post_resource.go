package resources

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/hooks"
	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/resource"
)

func RegisterPostRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	postHooks := &hooks.PostHooks{}

	baseResource := resource.NewBaseResource(
		db,
		models.Post{},
		paginationLimit,
		maxPaginationLimit,
		postHooks,
	)

	app.Get("/posts", func(c *fiber.Ctx) error {
		return baseResource.GetAll(c)
	})

	app.Get("/posts/:id", func(c *fiber.Ctx) error {
		return baseResource.GetByID(c)
	})

	app.Post("/posts", func(c *fiber.Ctx) error {
		return baseResource.Create(c)
	})

	app.Put("/posts/:id", func(c *fiber.Ctx) error {
		return baseResource.Update(c)
	})

	app.Delete("/posts/:id", func(c *fiber.Ctx) error {
		return baseResource.Delete(c)
	})
}

func GetPostBySlug(db database.Database, slug string) (*models.Post, error) {
	query := fmt.Sprintf("SELECT * FROM post WHERE slug = %s", db.Dialect().Placeholder(1))

	var post models.Post
	err := db.QueryRow(context.Background(), query, slug).Scan(
		&post.Id,
		&post.UserId,
		&post.Slug,
		&post.Status,
		&post.Title,
		&post.Content,
		&post.PublishedAt,
		&post.UpdatedAt,
		&post.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &post, nil
}
