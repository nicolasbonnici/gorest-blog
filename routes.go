package blog

import (
	"github.com/gofiber/fiber/v3"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/rbac"

	"github.com/nicolasbonnici/gorest-blog/hooks"
	"github.com/nicolasbonnici/gorest-blog/importer"
)

// roleAdmin is the only role allowed to drive content imports. Imports can
// create, overwrite and (with truncate) destroy posts in bulk, so the surface
// is restricted to administrators.
const roleAdmin = "admin"

func RegisterRoutes(router fiber.Router, db database.Database, config *Config, authMiddleware fiber.Handler) *hooks.PostHooks {
	postHooks := RegisterPostRoutes(router, db, config, authMiddleware)

	if config.EnableImporter {
		importer.SetServiceFactory(func(db database.Database, reporter importer.ProgressReporter) importer.ImportService {
			return NewImporterService(db, reporter)
		})

		// Gate the import endpoints behind authentication and the admin role.
		// Without this the routes were anonymous, allowing an unauthenticated
		// caller to bulk-import or (with truncate) wipe all posts.
		var guards []fiber.Handler
		if authMiddleware != nil {
			guards = []fiber.Handler{
				authMiddleware,
				rbac.RequireAuthenticated(),
				rbac.RequireRole(nil, roleAdmin, roleAdmin),
			}
		}
		importer.RegisterRoutes(router, db, guards...)
	}

	return postHooks
}
