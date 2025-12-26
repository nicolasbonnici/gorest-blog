package resources

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/response"
	auth "github.com/nicolasbonnici/gorest-auth"
)

type LikeResource struct {
	DB                 database.Database
	CRUD               *crud.CRUD[models.Like]
	PaginationLimit    int
	PaginationMaxLimit int
}

func RegisterLikeRoutes(app *fiber.App, db database.Database, paginationLimit, maxPaginationLimit int) {
	res := &LikeResource{
		DB:                 db,
		CRUD:               crud.New[models.Like](db),
		PaginationLimit:    paginationLimit,
		PaginationMaxLimit: maxPaginationLimit,
	}

	app.Get("/likes", res.List)
	app.Get("/likes/:id", res.Get)
	app.Post("/likes", res.Create)
	app.Put("/likes/:id", res.Update)
	app.Delete("/likes/:id", res.Delete)
}

func (r *LikeResource) List(c *fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.PaginationLimit, r.PaginationMaxLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	allowedFields := []string{"id", "liker_id", "liked_id", "likeable", "likeable_id", "liked_at", "updated_at", "created_at"}

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

func (r *LikeResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := r.CRUD.GetByID(auth.Context(c), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	return response.SendFormatted(c, 200, item)
}

func (r *LikeResource) Create(c *fiber.Ctx) error {
	var item models.Like
	if err := c.BodyParser(&item); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.LikerId = &user.UserID
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

func (r *LikeResource) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var item models.Like
	if err := c.BodyParser(&item); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.LikerId = &user.UserID
	}

	if err := r.CRUD.Update(auth.Context(c), id, item); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return response.SendFormatted(c, 200, item)
}

func (r *LikeResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := r.CRUD.Delete(auth.Context(c), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(204)
}
