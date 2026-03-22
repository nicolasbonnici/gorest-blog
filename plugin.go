package blog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/migrations"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/plugin"
)

type BlogPlugin struct {
	config     Config
	db         database.Database
	authPlugin plugin.Plugin
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

	if deps, ok := config[plugin.ConfigKeyDependencies].(map[string]plugin.Plugin); ok {
		if authPlugin, exists := deps["auth"]; exists {
			p.authPlugin = authPlugin
		}
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

	var authMiddleware fiber.Handler
	if p.authPlugin != nil {
		authMiddleware = p.authPlugin.Handler()
	}

	RegisterRoutes(app, p.db, &p.config, authMiddleware)
	return nil
}

func (p *BlogPlugin) MigrationSource() interface{} {
	return migrations.GetMigrations()
}

func (p *BlogPlugin) MigrationDependencies() []string {
	return []string{"auth", "translatable", "metrics", "rbac"}
}

func (p *BlogPlugin) Dependencies() []string {
	return []string{"auth", "translatable", "metrics", "rbac"}
}

func (p *BlogPlugin) GetOpenAPIResources() []plugin.OpenAPIResource {
	return []plugin.OpenAPIResource{{
		Name:          "post",
		PluralName:    "posts",
		BasePath:      "/posts",
		Tags:          []string{"Blog"},
		ResponseModel: models.Post{},
		CreateModel:   dtos.PostCreateDTO{},
		UpdateModel:   dtos.PostUpdateDTO{},
		Description:   "Blog post management",
	}}
}
