package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog/migrations"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/plugin"

	_ "github.com/nicolasbonnici/gorest-blog/importer/engines/devto"
)

type BlogPlugin struct {
	config Config
	db     database.Database
}

func NewPlugin() plugin.Plugin {
	return &BlogPlugin{}
}

func (p *BlogPlugin) Name() string {
	return "blog"
}

func (p *BlogPlugin) Initialize(config map[string]interface{}) error {
	p.config = DefaultConfig()

	if db, ok := config["database"].(database.Database); ok {
		p.db = db
		p.config.Database = db
	}

	if paginationLimit, ok := config["pagination_limit"].(int); ok {
		p.config.PaginationLimit = paginationLimit
	}

	if maxPaginationLimit, ok := config["max_pagination_limit"].(int); ok {
		p.config.MaxPaginationLimit = maxPaginationLimit
	}

	if enableImporter, ok := config["enable_importer"].(bool); ok {
		p.config.EnableImporter = enableImporter
	}

	return nil
}

func (p *BlogPlugin) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

func (p *BlogPlugin) SetupEndpoints(app *fiber.App) error {
	if p.db == nil {
		return nil
	}

	RegisterBlogRoutes(app, p.db, p.config.PaginationLimit, p.config.MaxPaginationLimit)

	if p.config.EnableImporter {
		RegisterImporterRoutes(app, p.db)
	}

	return nil
}

func (p *BlogPlugin) MigrationSource() interface{} {
	return migrations.GetMigrations()
}

func (p *BlogPlugin) MigrationDependencies() []string {
	return []string{"auth"}
}
