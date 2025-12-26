package resources

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/resource"
)

func RegisterCommentRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	baseResource := resource.NewBaseResource(
		db,
		models.Comment{},
		paginationLimit,
		maxPaginationLimit,
		nil,
	)

	app.Get("/comments", func(c *fiber.Ctx) error {
		return baseResource.GetAll(c)
	})

	app.Get("/comments/:id", func(c *fiber.Ctx) error {
		return baseResource.GetByID(c)
	})

	app.Post("/comments", func(c *fiber.Ctx) error {
		return baseResource.Create(c)
	})

	app.Put("/comments/:id", func(c *fiber.Ctx) error {
		return baseResource.Update(c)
	})

	app.Delete("/comments/:id", func(c *fiber.Ctx) error {
		return baseResource.Delete(c)
	})
}
