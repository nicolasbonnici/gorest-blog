package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest/database"

	"github.com/nicolasbonnici/gorest-blog/hooks"
	"github.com/nicolasbonnici/gorest-blog/importer"
)

func RegisterRoutes(router fiber.Router, db database.Database, config *Config, authMiddleware fiber.Handler) *hooks.PostHooks {
	postHooks := RegisterPostRoutes(router, db, config, authMiddleware)

	if config.EnableImporter {
		importer.SetServiceFactory(func(db database.Database, reporter importer.ProgressReporter) importer.ImportService {
			return NewImporterService(db, reporter)
		})
		importer.RegisterRoutes(router, db)
	}

	return postHooks
}
