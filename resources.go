package blog

import (
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/query"
	"github.com/nicolasbonnici/gorest/rbac"
	"github.com/nicolasbonnici/gorest/response"

	"github.com/nicolasbonnici/gorest-blog/converters"
	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/hooks"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/services"
)

type PostResource struct {
	db                 database.Database
	crud               *crud.CRUD[models.Post]
	hooks              *hooks.PostHooks
	converter          *converters.PostConverter
	config             *Config
	translationService *services.TranslationService
}

func RegisterPostRoutes(router fiber.Router, db database.Database, config *Config, authMiddleware fiber.Handler) *hooks.PostHooks {
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

	loadRoles := createRoleLoader(db, rbacConfig.RoleHierarchy)

	postHooks := hooks.NewPostHooks(db, voter)
	res := &PostResource{
		db:                 db,
		crud:               crud.New[models.Post](db),
		hooks:              postHooks,
		converter:          &converters.PostConverter{},
		config:             config,
		translationService: services.NewTranslationService(db),
	}

	router.Get("/posts", res.List)
	router.Get("/posts/:id", res.Get)

	requireWriter := requireRole(rbacConfig.RoleHierarchy, rbacConfig.SuperuserRole, "writer")
	requireWriterOrModerator := requireAnyRole(rbacConfig.RoleHierarchy, rbacConfig.SuperuserRole, "writer", "moderator")

	if authMiddleware != nil {
		router.Post("/posts", authMiddleware, loadRoles, requireWriter, res.Create)
		router.Put("/posts/:id", authMiddleware, loadRoles, requireWriterOrModerator, res.Update)
		router.Delete("/posts/:id", authMiddleware, loadRoles, requireWriter, res.Delete)
	} else {
		router.Post("/posts", loadRoles, requireWriter, res.Create)
		router.Put("/posts/:id", loadRoles, requireWriterOrModerator, res.Update)
		router.Delete("/posts/:id", loadRoles, requireWriter, res.Delete)
	}

	return postHooks
}

func (r *PostResource) List(c fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.config.PaginationLimit, r.config.MaxPaginationLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	queryParams := make(url.Values)
	for key, value := range c.Request().URI().QueryArgs().All() {
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

	ctx := c.Context()
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

func (r *PostResource) Get(c fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.Context()

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

	if err := r.hooks.CheckReadAccess(c, item); err != nil {
		return err
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

func (r *PostResource) Create(c fiber.Ctx) error {
	var dto dtos.PostCreateDTO
	if err := c.Bind().Body(&dto); err != nil {
		return response.SendError(c, fiber.StatusBadRequest, "invalid request body")
	}

	ctx := c.Context()

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

func (r *PostResource) Update(c fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.Context()

	var dto dtos.PostUpdateDTO
	if err := c.Bind().Body(&dto); err != nil {
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

func (r *PostResource) Delete(c fiber.Ctx) error {
	id := c.Params("id")

	if err := r.hooks.DeleteHook(c, id); err != nil {
		return err
	}

	if err := r.crud.Delete(c.Context(), id); err != nil {
		if crud.IsNotFoundError(err) {
			return response.SendError(c, fiber.StatusNotFound, "post not found")
		}
		return response.SendError(c, fiber.StatusInternalServerError, "database error")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func createRoleLoader(db database.Database, hierarchy map[string][]string) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		qb := query.New(db.Dialect())
		sql, args, err := qb.
			Select("r.name").
			From("user_roles").
			As("ur").
			JoinAs("roles", "r", query.ColEq("ur.role_id", "r.id")).
			Where(query.Eq("ur.user_id", userID)).
			Build()
		if err != nil {
			return c.Next()
		}

		rows, err := db.Query(c.Context(), sql, args...)
		if err != nil {
			return c.Next()
		}
		defer func() { _ = rows.Close() }()

		var roles []string
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				continue
			}
			roles = append(roles, role)
		}

		if len(roles) > 0 {
			c.Locals("user_roles", roles)
			userCtx := c.Context()
			userCtx = rbac.WithRoles(userCtx, roles)
			c.SetContext(userCtx)
		}

		return c.Next()
	}
}

func requireRole(hierarchy map[string][]string, superuserRole string, requiredRole string) fiber.Handler {
	return func(c fiber.Ctx) error {
		roles, ok := rbac.GetRoles(c.Context())
		if !ok || len(roles) == 0 {
			return response.SendError(c, fiber.StatusForbidden, "insufficient permissions")
		}

		// Check if user has superuser role
		for _, role := range roles {
			if role == superuserRole {
				return c.Next()
			}
		}

		// Check if user has required role (with hierarchy)
		if rbac.HasRole(roles, requiredRole, hierarchy) {
			return c.Next()
		}

		return response.SendError(c, fiber.StatusForbidden, "insufficient permissions")
	}
}

func requireAnyRole(hierarchy map[string][]string, superuserRole string, requiredRoles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		roles, ok := rbac.GetRoles(c.Context())
		if !ok || len(roles) == 0 {
			return response.SendError(c, fiber.StatusForbidden, "insufficient permissions")
		}

		// Check if user has superuser role
		for _, role := range roles {
			if role == superuserRole {
				return c.Next()
			}
		}

		// Check if user has any of the required roles (with hierarchy)
		if rbac.HasAnyRole(roles, requiredRoles, hierarchy) {
			return c.Next()
		}

		return response.SendError(c, fiber.StatusForbidden, "insufficient permissions")
	}
}
