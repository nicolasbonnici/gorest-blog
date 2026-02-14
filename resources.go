package blog

import (
	"context"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	auth "github.com/nicolasbonnici/gorest-auth"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest-rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/response"
)

type PostResource struct {
	DB                 database.Database
	CRUD               *crud.CRUD[Post]
	Config             *Config
	PaginationLimit    int
	PaginationMaxLimit int
	Voter              rbac.Voter
}

func RegisterPostRoutes(app *fiber.App, db database.Database, config *Config) {
	rbacConfig := rbac.Config{
		DefaultPolicy:      rbac.DenyAll,
		SuperuserRole:      "admin",
		RoleHierarchy:      map[string][]string{
			"writer":    {"moderator"},
			"moderator": {"reader"},
		},
		CacheEnabled:       true,
		CacheTTL:           300,
		StrictMode:         false,
		DefaultFieldPolicy: "deny",
	}

	voter, err := rbac.NewVoter(rbacConfig)
	if err != nil {
		panic("failed to create RBAC voter: " + err.Error())
	}

	roleProvider := rbac.NewFiberRoleProvider("user_roles", "user_id")

	res := &PostResource{
		DB:                 db,
		CRUD:               crud.New[Post](db),
		Config:             config,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
		Voter:              voter,
	}

	app.Get("/posts", res.List)
	app.Get("/posts/:id", res.Get)
	app.Post("/posts", rbac.RequireRole(voter, roleProvider, "writer"), res.Create)
	app.Put("/posts/:id", rbac.RequireRole(voter, roleProvider, "writer", "moderator"), res.Update)
	app.Delete("/posts/:id", rbac.RequireRole(voter, roleProvider, "writer"), res.Delete)
}

func (r *PostResource) List(c *fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.PaginationLimit, r.PaginationMaxLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	allowedFields := []string{"id", "user_id", "slug", "status", "published_at", "updated_at", "created_at"}

	queryParams := make(url.Values)
	for key, value := range c.Context().QueryArgs().All() {
		queryParams.Add(string(key), string(value))
	}

	filters := filter.NewFilterSet(allowedFields, r.DB.Dialect())
	if err := filters.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	ordering := filter.NewOrderSet(allowedFields)
	if err := ordering.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	filterOrderClauses := ordering.OrderClauses()
	orderByClauses := make([]crud.OrderByClause, len(filterOrderClauses))
	for i, oc := range filterOrderClauses {
		orderByClauses[i] = crud.OrderByClause{
			Column:    oc.Column,
			Direction: oc.Direction,
		}
	}

	ctx := auth.Context(c)

	translationService := NewTranslationService(r.DB)
	result, err := translationService.LoadPostsWithTranslations(ctx, limit, offset, includeCount, filters.Conditions(), orderByClauses)
	if err != nil {
		return pagination.SendPaginatedError(c, 500, err.Error())
	}

	items := make([]interface{}, len(result.Posts))
	for i, post := range result.Posts {
		filtered, err := r.Voter.FilterRead(ctx, post)
		if err != nil {
			items[i] = *post
		} else {
			items[i] = filtered
		}
	}

	return pagination.SendHydraCollection(c, items, result.Total, limit, page, r.PaginationLimit)
}

func (r *PostResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := auth.Context(c)

	item, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	translationService := NewTranslationService(r.DB)
	translations, err := translationService.ListTranslations(ctx, id)
	if err == nil && len(translations) > 0 {
		item.Translations = translations
	}

	metricsService := NewMetricsService(r.DB)
	metrics, err := metricsService.GetMetrics(ctx, id)
	if err == nil {
		item.Metrics = metrics
	}

	// Asynchronously increment view count
	go func() {
		bgCtx := context.Background()
		_ = metricsService.IncrementViews(bgCtx, id)
	}()

	// Apply RBAC filtering
	filtered, err := r.Voter.FilterRead(ctx, item)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to filter response"})
	}

	return response.SendFormatted(c, 200, filtered)
}

func (r *PostResource) Create(c *fiber.Ctx) error {
	var req CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := req.Validate(); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := auth.Context(c)

	// Build item with user-provided fields for validation
	var item Post
	item.Slug = req.Slug
	item.Status = req.Status

	// Validate user-provided fields
	if err := r.Voter.ValidateWrite(ctx, &item); err != nil {
		return c.Status(403).JSON(fiber.Map{"error": err.Error()})
	}

	// Set system-generated fields after validation
	item.Id = uuid.New().String()

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	if req.Status == types.PostStatusPublished {
		now := time.Now()
		item.PublishedAt = &now
	}

	// Create post record without title/content
	if err := r.CRUD.Create(ctx, item); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Create translations for all locales
	translationService := NewTranslationService(r.DB)
	var userUUID *uuid.UUID
	if item.UserId != nil {
		parsed, err := uuid.Parse(*item.UserId)
		if err == nil {
			userUUID = &parsed
		}
	}
	if err := translationService.CreateTranslations(ctx, item.Id, req.Translations, userUUID); err != nil {
		_ = r.CRUD.Delete(ctx, item.Id)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create translations: " + err.Error()})
	}

	metricsService := NewMetricsService(r.DB)
	if err := metricsService.InitializeMetrics(ctx, item.Id); err != nil {
		_ = translationService.DeleteAllTranslations(ctx, item.Id)
		_ = r.CRUD.Delete(ctx, item.Id)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to initialize metrics: " + err.Error()})
	}

	created, err := r.CRUD.GetByID(ctx, item.Id)
	if err != nil {
		filtered, filterErr := r.Voter.FilterRead(ctx, &item)
		if filterErr != nil {
			return response.SendFormatted(c, 201, item)
		}
		return response.SendFormatted(c, 201, filtered)
	}

	translations, err := translationService.ListTranslations(ctx, item.Id)
	if err == nil && len(translations) > 0 {
		created.Translations = translations
	}

	metrics, err := metricsService.GetMetrics(ctx, item.Id)
	if err == nil {
		created.Metrics = metrics
	}

	filtered, err := r.Voter.FilterRead(ctx, created)
	if err != nil {
		return response.SendFormatted(c, 201, created)
	}

	return response.SendFormatted(c, 201, filtered)
}

func (r *PostResource) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var req UpdatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := req.Validate(); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := auth.Context(c)

	// Fetch existing post
	existing, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	// Create update object for validation
	updateItem := *existing
	if req.Slug != "" {
		updateItem.Slug = req.Slug
	}
	if req.Status != "" {
		updateItem.Status = req.Status
		if req.Status == types.PostStatusPublished && updateItem.PublishedAt == nil {
			now := time.Now()
			updateItem.PublishedAt = &now
		}
	}
	if req.PublishedAt != nil {
		updateItem.PublishedAt = req.PublishedAt
	}

	// Clear read-only fields before RBAC validation
	updateItem.Id = ""
	updateItem.CreatedAt = nil
	updateItem.UpdatedAt = nil
	updateItem.Metrics = nil

	if err := r.Voter.ValidateWrite(ctx, &updateItem); err != nil {
		return c.Status(403).JSON(fiber.Map{"error": err.Error()})
	}

	// Apply validated changes
	existing.Slug = updateItem.Slug
	existing.Status = updateItem.Status
	existing.PublishedAt = updateItem.PublishedAt
	now := time.Now()
	existing.UpdatedAt = &now

	if err := r.CRUD.Update(ctx, id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update translations if provided
	if len(req.Translations) > 0 {
		translationService := NewTranslationService(r.DB)
		var userUUID *uuid.UUID
		if existing.UserId != nil {
			parsed, err := uuid.Parse(*existing.UserId)
			if err == nil {
				userUUID = &parsed
			}
		}

		// Update each translation
		for locale, translation := range req.Translations {
			if err := translationService.UpdateTranslation(ctx, id, locale, translation.Title, translation.Content, userUUID); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to update translation for locale " + locale + ": " + err.Error()})
			}
		}
	}

	// Fetch post with all translations and metrics
	updated, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		updated = existing
	}

	translationService := NewTranslationService(r.DB)
	translations, err := translationService.ListTranslations(ctx, id)
	if err == nil && len(translations) > 0 {
		updated.Translations = translations
	}

	metricsService := NewMetricsService(r.DB)
	metrics, err := metricsService.GetMetrics(ctx, id)
	if err == nil {
		updated.Metrics = metrics
	}

	filtered, err := r.Voter.FilterRead(ctx, updated)
	if err != nil {
		return response.SendFormatted(c, 200, updated)
	}

	return response.SendFormatted(c, 200, filtered)
}

func (r *PostResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := auth.Context(c)

	// Delete all translations first
	translationService := NewTranslationService(r.DB)
	if err := translationService.DeleteAllTranslations(ctx, id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete translations: " + err.Error()})
	}

	// Delete the post
	if err := r.CRUD.Delete(ctx, id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(204)
}
