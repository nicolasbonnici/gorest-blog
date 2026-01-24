package blog

import (
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
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

	allowedFields := []string{"id", "user_id", "slug", "status", "title", "content", "published_at", "updated_at", "created_at"}

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

	result, err := r.CRUD.GetAllPaginated(auth.Context(c), crud.PaginationOptions{
		Limit:        limit,
		Offset:       offset,
		IncludeCount: includeCount,
		Conditions:   filters.Conditions(),
		OrderBy:      orderByClauses,
	})
	if err != nil {
		return pagination.SendPaginatedError(c, 500, err.Error())
	}

	return pagination.SendHydraCollection(c, result.Items, result.Total, limit, page, r.PaginationLimit)
}

func (r *PostResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := r.CRUD.GetByID(auth.Context(c), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
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
	item.Slug = req.Slug
	item.Status = req.Status
	item.Title = req.Title
	item.Content = req.Content

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	if req.Status == types.PostStatusPublished {
		now := time.Now()
		item.PublishedAt = &now
	}

	ctx := auth.Context(c)
	if err := r.CRUD.Create(ctx, item); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	created, err := r.CRUD.GetByID(ctx, item.Id)
	if err != nil {
		return response.SendFormatted(c, 201, item)
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

	existing, err := r.CRUD.GetByID(auth.Context(c), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	if req.Slug != nil {
		existing.Slug = *req.Slug
	}
	if req.Status != nil {
		wasPublished := existing.Status == types.PostStatusPublished
		existing.Status = *req.Status

		if !wasPublished && *req.Status == types.PostStatusPublished {
			now := time.Now()
			existing.PublishedAt = &now
		}
	}
	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Content != nil {
		existing.Content = *req.Content
	}

	now := time.Now()
	existing.UpdatedAt = &now

	if err := r.CRUD.Update(auth.Context(c), id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return response.SendFormatted(c, 200, existing)
}

func (r *PostResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := r.CRUD.Delete(auth.Context(c), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(204)
}
