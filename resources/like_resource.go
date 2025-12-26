package resources

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/resource"
)

func RegisterLikeRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	baseResource := resource.NewBaseResource(
		db,
		models.Like{},
		paginationLimit,
		maxPaginationLimit,
		nil,
	)

	app.Get("/likes", func(c *fiber.Ctx) error {
		return baseResource.GetAll(c)
	})

	app.Get("/likes/:id", func(c *fiber.Ctx) error {
		return baseResource.GetByID(c)
	})

	app.Post("/likes", func(c *fiber.Ctx) error {
		return baseResource.Create(c)
	})

	app.Put("/likes/:id", func(c *fiber.Ctx) error {
		return baseResource.Update(c)
	})

	app.Delete("/likes/:id", func(c *fiber.Ctx) error {
		return baseResource.Delete(c)
	})
}
