package blog

import (
	"github.com/gofiber/fiber/v3"
	gorestconfig "github.com/nicolasbonnici/gorest/config"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/plugin"

	"github.com/nicolasbonnici/gorest/auth/jwt"
	authmiddleware "github.com/nicolasbonnici/gorest/auth/middleware"

	ai "github.com/nicolasbonnici/gorest-ai"
	taxonomy "github.com/nicolasbonnici/gorest-taxonomy"

	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/hooks"
	"github.com/nicolasbonnici/gorest-blog/migrations"
	"github.com/nicolasbonnici/gorest-blog/models"
)

type BlogPlugin struct {
	config          Config
	db              database.Database
	authMiddleware  fiber.Handler
	postHooks       *hooks.PostHooks
	taxonomyService *taxonomy.TaxonomyService
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

	if appCfg, ok := config["config"].(*gorestconfig.Config); ok && appCfg.Auth.Enabled && p.db != nil {
		jwtSvc := jwt.NewService(appCfg.Auth.JWTSecret, appCfg.Auth.JWTTTL)
		p.authMiddleware = authmiddleware.OptionalAuthMiddleware(jwtSvc, p.db)
	}

	if deps, ok := config[plugin.ConfigKeyDependencies].(map[string]plugin.Plugin); ok {
		if taxPlugin, ok := deps["taxonomy"]; ok {
			if provider, ok := taxPlugin.(taxonomy.ServiceProvider); ok {
				p.taxonomyService = provider.GetService()
			}
		}
	}

	return nil
}

func (p *BlogPlugin) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Next()
	}
}

func (p *BlogPlugin) SetupEndpoints(router fiber.Router) error {
	if p.db == nil {
		return nil
	}

	p.postHooks = RegisterRoutes(router, p.db, &p.config, p.authMiddleware)
	if p.taxonomyService != nil {
		p.postHooks.SetTaxonomyService(p.taxonomyService)
	}
	return nil
}

func (p *BlogPlugin) SetAutoTranslator(at *ai.AutoTranslator) {
	if p.postHooks != nil {
		p.postHooks.SetAutoTranslator(at)
	}
}

func (p *BlogPlugin) MigrationSource() interface{} {
	return migrations.GetMigrations()
}

func (p *BlogPlugin) MigrationDependencies() []string {
	return []string{"translatable", "metrics"}
}

func (p *BlogPlugin) Dependencies() []string {
	return []string{"translatable", "metrics", "taxonomy"}
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
		ListQueryParams: []plugin.QueryParam{
			{
				Name:        "search",
				Description: "Filter posts by title (case-insensitive partial match across all locales)",
				Type:        "string",
			},
		},
	}}
}
