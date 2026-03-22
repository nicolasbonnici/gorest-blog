package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	auth "github.com/nicolasbonnici/gorest-auth"
	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/services"
	"github.com/nicolasbonnici/gorest-rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"
)

type PostHooks struct {
	db                 database.Database
	voter              rbac.Voter
	translationService *services.TranslationService
	metricsService     *services.MetricsService
}

func NewPostHooks(db database.Database, voter rbac.Voter) *PostHooks {
	return &PostHooks{
		db:                 db,
		voter:              voter,
		translationService: services.NewTranslationService(db),
		metricsService:     services.NewMetricsService(db),
	}
}

func (h *PostHooks) CreateHook(c *fiber.Ctx, dto dtos.PostCreateDTO, model *models.Post) error {
	if err := h.validateCreateDTO(dto); err != nil {
		return fiber.NewError(400, err.Error())
	}

	if err := h.enrichCreateModel(c, model); err != nil {
		return err
	}

	tempModel := *model
	tempModel.ID = ""
	tempModel.UserID = nil
	tempModel.RemoteSourceID = nil
	tempModel.RemoteSource = nil

	if err := h.voter.ValidateWrite(c.UserContext(), &tempModel); err != nil {
		return fiber.NewError(403, fmt.Sprintf("insufficient permissions: %v", err))
	}

	return nil
}

func (h *PostHooks) UpdateHook(c *fiber.Ctx, dto dtos.PostUpdateDTO, model *models.Post) error {
	if err := h.validateUpdateDTO(dto); err != nil {
		return fiber.NewError(400, err.Error())
	}

	userID, hasUser := c.Locals("user_id").(string)
	if hasUser && model.UserID != nil {
		userRoles, _ := c.Locals("user_roles").([]string)
		isOwner := *model.UserID == userID
		isModerator := h.hasAnyRole(userRoles, []string{"moderator", "admin"})

		if !isOwner && !isModerator {
			return fiber.NewError(403, "insufficient permissions: you can only update your own posts")
		}
	}

	tempModel := *model
	tempModel.ID = ""
	tempModel.UserID = nil
	tempModel.CreatedAt = nil
	tempModel.UpdatedAt = nil
	tempModel.Metrics = nil
	tempModel.RemoteSourceID = nil
	tempModel.RemoteSource = nil

	if err := h.voter.ValidateWrite(c.UserContext(), &tempModel); err != nil {
		return fiber.NewError(403, err.Error())
	}

	return nil
}

func (h *PostHooks) DeleteHook(c *fiber.Ctx, id any) error {
	postID, ok := id.(string)
	if !ok {
		return fiber.NewError(400, "invalid post ID")
	}

	ctx := c.UserContext()

	if err := h.translationService.DeleteAllTranslations(ctx, postID); err != nil {
		return fiber.NewError(500, "failed to delete translations: "+err.Error())
	}

	return nil
}

func (h *PostHooks) GetByIDHook(c *fiber.Ctx, id any) error {
	return nil
}

func (h *PostHooks) GetAllHook(c *fiber.Ctx, conditions *[]query.Condition, orderBy *[]crud.OrderByClause) error {
	return nil
}

func (h *PostHooks) AfterCreate(ctx context.Context, c *fiber.Ctx, dto dtos.PostCreateDTO, model *models.Post) error {
	modelTranslations := h.convertDTOTranslations(dto.Translations)

	var userUUID *uuid.UUID
	if model.UserID != nil {
		parsed, err := uuid.Parse(*model.UserID)
		if err == nil {
			userUUID = &parsed
		}
	}

	if err := h.translationService.CreateTranslations(ctx, model.ID, modelTranslations, userUUID); err != nil {
		_ = h.rollbackPost(ctx, model.ID)
		return fiber.NewError(500, "failed to create translations: "+err.Error())
	}

	if err := h.metricsService.InitializeMetrics(ctx, model.ID); err != nil {
		_ = h.translationService.DeleteAllTranslations(ctx, model.ID)
		_ = h.rollbackPost(ctx, model.ID)
		return fiber.NewError(500, "failed to initialize metrics: "+err.Error())
	}

	return nil
}

func (h *PostHooks) AfterUpdate(ctx context.Context, c *fiber.Ctx, dto dtos.PostUpdateDTO, model *models.Post) error {
	if len(dto.Translations) == 0 {
		return nil
	}

	var userUUID *uuid.UUID
	if model.UserID != nil {
		parsed, err := uuid.Parse(*model.UserID)
		if err == nil {
			userUUID = &parsed
		}
	}

	modelTranslations := h.convertDTOTranslations(dto.Translations)

	for locale, translation := range modelTranslations {
		if err := h.translationService.UpdateTranslation(ctx, model.ID, locale, translation.Title, translation.Content, userUUID); err != nil {
			return fiber.NewError(500, "failed to update translation for locale "+locale+": "+err.Error())
		}
	}

	return nil
}

func (h *PostHooks) EnrichGetByID(ctx context.Context, c *fiber.Ctx, model *models.Post) error {
	translations, err := h.translationService.ListTranslations(ctx, model.ID)
	if err == nil && len(translations) > 0 {
		model.Translations = translations
	}

	metrics, err := h.metricsService.GetMetrics(ctx, model.ID)
	if err == nil {
		model.Metrics = metrics
	}

	go func() {
		bgCtx := context.Background()
		_ = h.metricsService.IncrementViews(bgCtx, model.ID)
	}()

	return nil
}

func (h *PostHooks) EnrichGetAll(ctx context.Context, c *fiber.Ctx, models []*models.Post) error {
	for _, model := range models {
		translations, err := h.translationService.ListTranslations(ctx, model.ID)
		if err == nil && len(translations) > 0 {
			model.Translations = translations
		}

		metrics, err := h.metricsService.GetMetrics(ctx, model.ID)
		if err == nil {
			model.Metrics = metrics
		}
	}

	return nil
}

func (h *PostHooks) ApplyRBACFilter(ctx context.Context, model *models.Post) (interface{}, error) {
	filtered, err := h.voter.FilterRead(ctx, model)
	if err != nil {
		return model, nil
	}
	return filtered, nil
}

func (h *PostHooks) validateCreateDTO(dto dtos.PostCreateDTO) error {
	dto.Slug = strings.TrimSpace(dto.Slug)
	if dto.Slug == "" {
		return errors.New("slug cannot be empty")
	}

	if !dto.Status.IsValid() {
		return errors.New("invalid status value")
	}

	if len(dto.Translations) == 0 {
		return errors.New("at least one translation is required")
	}

	for locale, translation := range dto.Translations {
		if translation == nil {
			return errors.New("translation for locale " + locale + " cannot be nil")
		}

		if err := h.validateTranslation(locale, translation); err != nil {
			return err
		}

		h.sanitizeTranslation(translation)
	}

	return nil
}

func (h *PostHooks) validateUpdateDTO(dto dtos.PostUpdateDTO) error {
	if dto.Slug != nil {
		slug := strings.TrimSpace(*dto.Slug)
		if slug == "" {
			return errors.New("slug cannot be empty")
		}
		*dto.Slug = slug
	}

	if dto.Status != nil {
		if !dto.Status.IsValid() {
			return errors.New("invalid status value")
		}
	}

	if dto.Translations != nil {
		for locale, translation := range dto.Translations {
			if translation == nil {
				return errors.New("translation for locale " + locale + " cannot be nil")
			}

			if err := h.validateTranslation(locale, translation); err != nil {
				return err
			}

			h.sanitizeTranslation(translation)
		}
	}

	return nil
}

func (h *PostHooks) validateTranslation(locale string, translation *dtos.PostTranslationContentDTO) error {
	modelTranslation := &models.PostTranslationContent{
		Title:   translation.Title,
		Content: translation.Content,
	}

	if err := modelTranslation.Validate(); err != nil {
		return errors.New("validation failed for locale " + locale + ": " + err.Error())
	}

	translation.Title = modelTranslation.Title
	translation.Content = modelTranslation.Content

	return nil
}

func (h *PostHooks) sanitizeTranslation(translation *dtos.PostTranslationContentDTO) {
	modelTranslation := &models.PostTranslationContent{
		Title:   translation.Title,
		Content: translation.Content,
	}

	modelTranslation.Sanitize()

	translation.Title = modelTranslation.Title
	translation.Content = modelTranslation.Content
}

func (h *PostHooks) enrichCreateModel(c *fiber.Ctx, model *models.Post) error {
	if user := auth.GetAuthenticatedUser(c); user != nil {
		model.UserID = &user.UserID
	}

	return nil
}

func (h *PostHooks) convertDTOTranslations(dtoTranslations map[string]*dtos.PostTranslationContentDTO) map[string]*models.PostTranslationContent {
	if dtoTranslations == nil {
		return nil
	}

	modelTranslations := make(map[string]*models.PostTranslationContent)
	for locale, dto := range dtoTranslations {
		if dto != nil {
			modelTranslations[locale] = &models.PostTranslationContent{
				Title:   dto.Title,
				Content: dto.Content,
			}
		}
	}
	return modelTranslations
}

func (h *PostHooks) rollbackPost(ctx context.Context, postID string) error {
	sql, args, err := query.New(h.db.Dialect()).
		Delete("post").
		Where(query.Eq("id", postID)).
		Build()
	if err != nil {
		return err
	}

	_, err = h.db.Exec(ctx, sql, args...)
	return err
}

func (h *PostHooks) hasAnyRole(userRoles []string, requiredRoles []string) bool {
	for _, userRole := range userRoles {
		for _, requiredRole := range requiredRoles {
			if userRole == requiredRole {
				return true
			}
		}
	}
	return false
}
