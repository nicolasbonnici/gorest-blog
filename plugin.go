package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/plugin"
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
	return nil
}

func (p *BlogPlugin) MigrationDependencies() []string {
	return []string{"auth"}
}

func (p *BlogPlugin) Dependencies() []string {
	return []string{"auth"}
}
