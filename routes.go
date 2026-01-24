package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest-blog/importer"
)

func RegisterRoutes(app *fiber.App, db database.Database, config *Config) {
	RegisterPostRoutes(app, db, config)

	if config.EnableImporter {
		importer.SetServiceFactory(func(db database.Database, reporter importer.ProgressReporter) importer.ImportService {
			return NewImporterService(db, reporter)
		})
		importer.RegisterRoutes(app, db)
	}
}
