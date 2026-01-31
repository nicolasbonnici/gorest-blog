package blog

import (
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	auth "github.com/nicolasbonnici/gorest-auth"
	"github.com/nicolasbonnici/gorest-blog/types"
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
}

func RegisterPostRoutes(app *fiber.App, db database.Database, config *Config) {
	res := &PostResource{
		DB:                 db,
		CRUD:               crud.New[Post](db),
		Config:             config,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
	}

	app.Get("/posts", res.List)
	app.Get("/posts/:id", res.Get)
	app.Post("/posts", res.Create)
	app.Put("/posts/:id", res.Update)
	app.Delete("/posts/:id", res.Delete)
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

	result, err := r.CRUD.GetAllPaginated(ctx, crud.PaginationOptions{
		Limit:        limit,
		Offset:       offset,
		IncludeCount: includeCount,
		Conditions:   filters.Conditions(),
		OrderBy:      orderByClauses,
	})
	if err != nil {
		return pagination.SendPaginatedError(c, 500, err.Error())
	}

	// Always fetch translations for all posts
	translationService := NewTranslationService(r.DB)
	for i := range result.Items {
		translations, err := translationService.ListTranslations(ctx, result.Items[i].Id)
		if err == nil && len(translations) > 0 {
			result.Items[i].Translations = translations
		}
	}

	return pagination.SendHydraCollection(c, result.Items, result.Total, limit, page, r.PaginationLimit)
}

func (r *PostResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := auth.Context(c)

	item, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	// Always fetch translations
	translationService := NewTranslationService(r.DB)
	translations, err := translationService.ListTranslations(ctx, id)
	if err == nil && len(translations) > 0 {
		item.Translations = translations
	}

	return response.SendFormatted(c, 200, item)
}

func (r *PostResource) Create(c *fiber.Ctx) error {
	var req CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := req.Validate(); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	var item Post
	item.Id = uuid.New().String() // Generate UUID before insert
	item.Slug = req.Slug
	item.Status = req.Status

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	if req.Status == types.PostStatusPublished {
		now := time.Now()
		item.PublishedAt = &now
	}

	ctx := auth.Context(c)

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
		// Rollback: delete the post if translation creation fails
		_ = r.CRUD.Delete(ctx, item.Id)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create translations: " + err.Error()})
	}

	// Fetch post with translations
	created, err := r.CRUD.GetByID(ctx, item.Id)
	if err != nil {
		return response.SendFormatted(c, 201, item)
	}

	translations, err := translationService.ListTranslations(ctx, item.Id)
	if err == nil && len(translations) > 0 {
		created.Translations = translations
	}

	return response.SendFormatted(c, 201, created)
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

	// Fetch existing post to verify ownership
	existing, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	// Update the translation for the specified locale
	translationService := NewTranslationService(r.DB)
	var userUUID *uuid.UUID
	if existing.UserId != nil {
		parsed, err := uuid.Parse(*existing.UserId)
		if err == nil {
			userUUID = &parsed
		}
	}
	if err := translationService.UpdateTranslation(ctx, id, req.Locale, req.Title, req.Content, userUUID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update translation: " + err.Error()})
	}

	// Update post's updated_at timestamp
	now := time.Now()
	existing.UpdatedAt = &now

	if err := r.CRUD.Update(ctx, id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Fetch post with all translations
	updated, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		updated = existing
	}

	translations, err := translationService.ListTranslations(ctx, id)
	if err == nil && len(translations) > 0 {
		updated.Translations = translations
	}

	return response.SendFormatted(c, 200, updated)
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
