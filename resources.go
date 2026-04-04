package blog

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog/converters"
	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/hooks"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/services"
	"github.com/nicolasbonnici/gorest-rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/query"
	"github.com/nicolasbonnici/gorest/response"
)

type PostResource struct {
	db                 database.Database
	crud               *crud.CRUD[models.Post]
	hooks              *hooks.PostHooks
	converter          *converters.PostConverter
	config             *Config
	translationService *services.TranslationService
}

func RegisterPostRoutes(app *fiber.App, db database.Database, config *Config, authMiddleware fiber.Handler) {
	rbacConfig := rbac.Config{
		DefaultPolicy: rbac.DenyAll,
		SuperuserRole: "admin",
		RoleHierarchy: map[string][]string{
			"moderator": {"writer"},
			"writer":    {"reader"},
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

	loadRoles := createRoleLoader(db)
	roleProvider := rbac.NewFiberRoleProvider("user_roles", "user_id")

	res := &PostResource{
		db:                 db,
		crud:               crud.New[models.Post](db),
		hooks:              hooks.NewPostHooks(db, voter),
		converter:          &converters.PostConverter{},
		config:             config,
		translationService: services.NewTranslationService(db),
	}

	app.Get("/posts", res.List)
	app.Get("/posts/:id", res.Get)

	if authMiddleware != nil {
		app.Post("/posts", authMiddleware, loadRoles, rbac.RequireRole(voter, roleProvider, "writer"), res.Create)
		app.Put("/posts/:id", authMiddleware, loadRoles, rbac.RequireRole(voter, roleProvider, "writer", "moderator"), res.Update)
		app.Delete("/posts/:id", authMiddleware, loadRoles, rbac.RequireRole(voter, roleProvider, "writer"), res.Delete)
	} else {
		app.Post("/posts", loadRoles, rbac.RequireRole(voter, roleProvider, "writer"), res.Create)
		app.Put("/posts/:id", loadRoles, rbac.RequireRole(voter, roleProvider, "writer", "moderator"), res.Update)
		app.Delete("/posts/:id", loadRoles, rbac.RequireRole(voter, roleProvider, "writer"), res.Delete)
	}
}

func (r *PostResource) List(c *fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.config.PaginationLimit, r.config.MaxPaginationLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	queryParams := make(url.Values)
	for key, value := range c.Context().QueryArgs().All() {
		queryParams.Add(string(key), string(value))
	}

	fieldMap := map[string]string{
		"id":           "id",
		"user_id":      "user_id",
		"userId":       "user_id",
		"slug":         "slug",
		"status":       "status",
		"published_at": "published_at",
		"publishedAt":  "published_at",
		"updated_at":   "updated_at",
		"updatedAt":    "updated_at",
		"created_at":   "created_at",
		"createdAt":    "created_at",
	}

	var conditions []query.Condition
	filters := filter.NewFilterSetWithMapping(fieldMap, r.db.Dialect())
	if err := filters.ParseFromQuery(queryParams); err != nil {
		return response.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	conditions = filters.Conditions()

	var orderBy []crud.OrderByClause
	ordering := filter.NewOrderSetWithMapping(fieldMap)
	if err := ordering.ParseFromQuery(queryParams); err != nil {
		return response.SendError(c, fiber.StatusBadRequest, err.Error())
	}

	orderClauses := ordering.OrderClauses()
	orderBy = make([]crud.OrderByClause, len(orderClauses))
	for i, oc := range orderClauses {
		orderBy[i] = crud.OrderByClause{
			Column:    oc.Column,
			Direction: oc.Direction,
		}
	}

	if err := r.hooks.GetAllHook(c, &conditions, &orderBy); err != nil {
		return err
	}

	ctx := c.UserContext()
	result, err := r.translationService.LoadPostsWithTranslations(ctx, limit, offset, includeCount, conditions, orderBy)
	if err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, err.Error())
	}

	if err := r.hooks.EnrichGetAll(ctx, c, result.Posts); err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, "failed to enrich posts")
	}

	items := make([]interface{}, len(result.Posts))
	for i, post := range result.Posts {
		filtered, err := r.hooks.ApplyRBACFilter(ctx, post)
		if err != nil {
			items[i] = r.converter.ModelToResponseDTO(*post)
		} else {
			items[i] = filtered
		}
	}

	return pagination.SendHydraCollection(c, items, result.Total, limit, page, r.config.PaginationLimit)
}

func (r *PostResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()

	if err := r.hooks.GetByIDHook(c, id); err != nil {
		return err
	}

	item, err := r.crud.GetByID(ctx, id)
	if crud.IsNotFoundError(err) {
		return response.SendError(c, fiber.StatusNotFound, "post not found")
	}
	if err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	if err := r.hooks.EnrichGetByID(ctx, c, item); err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, "failed to enrich post")
	}

	filtered, err := r.hooks.ApplyRBACFilter(ctx, item)
	if err != nil {
		return response.SendFormatted(c, fiber.StatusOK, r.converter.ModelToResponseDTO(*item))
	}

	return response.SendFormatted(c, fiber.StatusOK, filtered)
}

func (r *PostResource) Create(c *fiber.Ctx) error {
	var dto dtos.PostCreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return response.SendError(c, fiber.StatusBadRequest, "invalid request body")
	}

	ctx := c.UserContext()

	var userID *string
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		userID = &uid
	}

	model := r.converter.CreateDTOToModel(dto, userID)

	if err := r.hooks.CreateHook(c, dto, &model); err != nil {
		return err
	}

	if err := r.crud.Create(ctx, model); err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	if err := r.hooks.AfterCreate(ctx, c, dto, &model); err != nil {
		return err
	}

	created, err := r.crud.GetByID(ctx, model.ID)
	if err == nil {
		if err := r.hooks.EnrichGetByID(ctx, c, created); err == nil {
			model = *created
		}
	}

	filtered, err := r.hooks.ApplyRBACFilter(ctx, &model)
	if err != nil {
		return response.SendFormatted(c, fiber.StatusCreated, r.converter.ModelToResponseDTO(model))
	}

	return response.SendFormatted(c, fiber.StatusCreated, filtered)
}

func (r *PostResource) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()

	var dto dtos.PostUpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return response.SendError(c, fiber.StatusBadRequest, "invalid request body")
	}

	existing, err := r.crud.GetByID(ctx, id)
	if crud.IsNotFoundError(err) {
		return response.SendError(c, fiber.StatusNotFound, "post not found")
	}
	if err != nil {
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	model := r.converter.UpdateDTOToModel(dto, existing)

	if err := r.hooks.UpdateHook(c, dto, &model); err != nil {
		return err
	}

	if err := r.crud.Update(ctx, id, model); err != nil {
		if crud.IsNotFoundError(err) {
			return response.SendError(c, fiber.StatusNotFound, "post not found")
		}
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	if err := r.hooks.AfterUpdate(ctx, c, dto, &model); err != nil {
		return err
	}

	updated, err := r.crud.GetByID(ctx, id)
	if err == nil {
		if err := r.hooks.EnrichGetByID(ctx, c, updated); err == nil {
			model = *updated
		}
	}

	filtered, err := r.hooks.ApplyRBACFilter(ctx, &model)
	if err != nil {
		return response.SendFormatted(c, fiber.StatusOK, r.converter.ModelToResponseDTO(model))
	}

	return response.SendFormatted(c, fiber.StatusOK, filtered)
}

func (r *PostResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := r.hooks.DeleteHook(c, id); err != nil {
		return err
	}

	if err := r.crud.Delete(c.UserContext(), id); err != nil {
		if crud.IsNotFoundError(err) {
			return response.SendError(c, fiber.StatusNotFound, "post not found")
		}
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func createRoleLoader(db database.Database) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		repo := rbac.NewRepository(db)
		roles, err := repo.GetUserRoles(c.UserContext(), userID)

		if err != nil || len(roles) == 0 {
			return c.Next()
		}

		c.Locals("user_roles", roles)

		userCtx := c.UserContext()
		userCtx = rbac.WithRoles(userCtx, roles)
		c.SetUserContext(userCtx)

		return c.Next()
	}
}
