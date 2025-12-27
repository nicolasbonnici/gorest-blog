package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest/database"
)

func RegisterImporterRoutes(app *fiber.App, db database.Database) {
	importer.RegisterRoutes(app, db)
}
