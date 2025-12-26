package resources

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/hooks"
	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/response"
	auth "github.com/nicolasbonnici/gorest-auth"
)

type PostResource struct {
	DB                 database.Database
	CRUD               *crud.CRUD[models.Post]
	PaginationLimit    int
	PaginationMaxLimit int
}

func RegisterPostRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	postHooks := &hooks.PostHooks{}

	res := &PostResource{
		DB:                 db,
		CRUD:               crud.NewWithHooks[models.Post](db, postHooks),
		PaginationLimit:    paginationLimit,
		PaginationMaxLimit: maxPaginationLimit,
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
	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
		queryParams.Add(string(key), string(value))
	})

	filters := filter.NewFilterSet(allowedFields, r.DB.Dialect())
	if err := filters.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}
	whereClause, whereArgs := filters.BuildWhereClause()

	ordering := filter.NewOrderSet(allowedFields)
	if err := ordering.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}
	orderByClause := ordering.BuildOrderByClause()

	result, err := r.CRUD.GetAllPaginated(auth.Context(c), crud.PaginationOptions{
		Limit:         limit,
		Offset:        offset,
		IncludeCount:  includeCount,
		WhereClause:   whereClause,
		WhereArgs:     whereArgs,
		OrderByClause: orderByClause,
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
	var item models.Post
	if err := c.BodyParser(&item); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
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
	var item models.Post
	if err := c.BodyParser(&item); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	if err := r.CRUD.Update(auth.Context(c), id, item); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return response.SendFormatted(c, 200, item)
}

func (r *PostResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := r.CRUD.Delete(auth.Context(c), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(204)
}

func GetPostBySlug(db database.Database, slug string) (*models.Post, error) {
	query := fmt.Sprintf("SELECT * FROM post WHERE slug = %s", db.Dialect().Placeholder(1))

	var post models.Post
	err := db.QueryRow(context.Background(), query, slug).Scan(
		&post.Id,
		&post.UserId,
		&post.Slug,
		&post.Status,
		&post.Title,
		&post.Content,
		&post.PublishedAt,
		&post.UpdatedAt,
		&post.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &post, nil
}
