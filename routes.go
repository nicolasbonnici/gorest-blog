package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/resources"
	"github.com/nicolasbonnici/gorest/database"
)

func RegisterBlogRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	resources.RegisterPostRoutes(app, db, paginationLimit, maxPaginationLimit)
	resources.RegisterCommentRoutes(app, db, paginationLimit, maxPaginationLimit)
	resources.RegisterLikeRoutes(app, db, paginationLimit, maxPaginationLimit)
}
