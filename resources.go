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
		DefaultPolicy: rbac.DenyAll,
		SuperuserRole: "admin",
		RoleHierarchy: map[string][]string{
			"writer":    {"moderator"},
			"moderator": {"reader"},
		},
		CacheEnabled:       true,
		CacheTTL:           300,
		StrictMode:         false,
		DefaultFieldPolicy: "allow",
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

	ctx := c.UserContext()

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
	ctx := c.UserContext()

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

	ctx := c.UserContext()

	item := r.buildPostFromRequest(c, &req)

	// Clear read-only fields before RBAC validation
	tempId := item.Id
	item.Id = ""
	item.RemoteSourceID = nil
	item.RemoteSource = nil

	if err := r.Voter.ValidateWrite(ctx, &item); err != nil {
		return c.Status(403).JSON(fiber.Map{"error": err.Error()})
	}

	// Restore ID for creation
	item.Id = tempId

	if err := r.createPostWithDependencies(ctx, &item, req.Translations); err != nil {
		return err
	}

	return r.buildCreateResponse(c, ctx, &item)
}

func (r *PostResource) buildPostFromRequest(c *fiber.Ctx, req *CreatePostRequest) Post {
	item := Post{
		Id:     uuid.New().String(),
		Slug:   req.Slug,
		Status: req.Status,
	}

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	if req.Status == types.PostStatusPublished {
		now := time.Now()
		item.PublishedAt = &now
	}

	return item
}

func (r *PostResource) createPostWithDependencies(ctx context.Context, item *Post, translations map[string]*PostTranslationContent) error {
	if err := r.CRUD.Create(ctx, *item); err != nil {
		return fiber.NewError(500, err.Error())
	}

	translationService := NewTranslationService(r.DB)
	var userUUID *uuid.UUID
	if item.UserId != nil {
		parsed, err := uuid.Parse(*item.UserId)
		if err == nil {
			userUUID = &parsed
		}
	}

	if err := translationService.CreateTranslations(ctx, item.Id, translations, userUUID); err != nil {
		_ = r.CRUD.Delete(ctx, item.Id)
		return fiber.NewError(500, "Failed to create translations: "+err.Error())
	}

	metricsService := NewMetricsService(r.DB)
	if err := metricsService.InitializeMetrics(ctx, item.Id); err != nil {
		_ = translationService.DeleteAllTranslations(ctx, item.Id)
		_ = r.CRUD.Delete(ctx, item.Id)
		return fiber.NewError(500, "Failed to initialize metrics: "+err.Error())
	}

	return nil
}

func (r *PostResource) buildCreateResponse(c *fiber.Ctx, ctx context.Context, item *Post) error {
	created, err := r.CRUD.GetByID(ctx, item.Id)
	if err != nil {
		return r.sendFilteredResponse(c, ctx, item, 201)
	}

	translationService := NewTranslationService(r.DB)
	translations, err := translationService.ListTranslations(ctx, item.Id)
	if err == nil && len(translations) > 0 {
		created.Translations = translations
	}

	metricsService := NewMetricsService(r.DB)
	metrics, err := metricsService.GetMetrics(ctx, item.Id)
	if err == nil {
		created.Metrics = metrics
	}

	return r.sendFilteredResponse(c, ctx, created, 201)
}

func (r *PostResource) sendFilteredResponse(c *fiber.Ctx, ctx context.Context, post *Post, statusCode int) error {
	filtered, err := r.Voter.FilterRead(ctx, post)
	if err != nil {
		return response.SendFormatted(c, statusCode, post)
	}
	return response.SendFormatted(c, statusCode, filtered)
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

	ctx := c.UserContext()

	existing, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	if err := r.validateAndApplyUpdates(ctx, existing, &req); err != nil {
		return err
	}

	if err := r.CRUD.Update(ctx, id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := r.updateTranslations(ctx, id, existing.UserId, req.Translations); err != nil {
		return err
	}

	return r.buildUpdateResponse(c, ctx, id, existing)
}

func (r *PostResource) validateAndApplyUpdates(ctx context.Context, existing *Post, req *UpdatePostRequest) error {
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
	updateItem.RemoteSourceID = nil
	updateItem.RemoteSource = nil

	if err := r.Voter.ValidateWrite(ctx, &updateItem); err != nil {
		return fiber.NewError(403, err.Error())
	}

	// Apply validated changes
	existing.Slug = updateItem.Slug
	existing.Status = updateItem.Status
	existing.PublishedAt = updateItem.PublishedAt
	now := time.Now()
	existing.UpdatedAt = &now

	return nil
}

func (r *PostResource) updateTranslations(ctx context.Context, postID string, userID *string, translations map[string]*PostTranslationContent) error {
	if len(translations) == 0 {
		return nil
	}

	translationService := NewTranslationService(r.DB)
	var userUUID *uuid.UUID
	if userID != nil {
		parsed, err := uuid.Parse(*userID)
		if err == nil {
			userUUID = &parsed
		}
	}

	for locale, translation := range translations {
		if err := translationService.UpdateTranslation(ctx, postID, locale, translation.Title, translation.Content, userUUID); err != nil {
			return fiber.NewError(500, "Failed to update translation for locale "+locale+": "+err.Error())
		}
	}

	return nil
}

func (r *PostResource) buildUpdateResponse(c *fiber.Ctx, ctx context.Context, id string, existing *Post) error {
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
	ctx := c.UserContext()

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
