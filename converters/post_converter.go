package converters

import (
	"time"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/dtos"
	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/types"
)

type PostConverter struct{}

func (c *PostConverter) CreateDTOToModel(dto dtos.PostCreateDTO, userID *string) models.Post {
	post := models.Post{
		ID:     uuid.New().String(),
		UserID: userID,
		Slug:   dto.Slug,
		Status: dto.Status,
	}

	if dto.Status == types.PostStatusPublished {
		now := time.Now()
		post.PublishedAt = &now
	}

	return post
}

func (c *PostConverter) UpdateDTOToModel(dto dtos.PostUpdateDTO, existing *models.Post) models.Post {
	updated := *existing

	if dto.Slug != nil {
		updated.Slug = *dto.Slug
	}

	if dto.Status != nil {
		updated.Status = *dto.Status
		if *dto.Status == types.PostStatusPublished && updated.PublishedAt == nil {
			now := time.Now()
			updated.PublishedAt = &now
		}
	}

	if dto.PublishedAt != nil {
		updated.PublishedAt = dto.PublishedAt
	}

	now := time.Now()
	updated.UpdatedAt = &now

	return updated
}

func (c *PostConverter) ModelToResponseDTO(model models.Post) dtos.PostResponseDTO {
	dto := dtos.PostResponseDTO{
		ID:             model.ID,
		UserID:         model.UserID,
		Slug:           model.Slug,
		Status:         model.Status,
		PublishedAt:    model.PublishedAt,
		RemoteSourceID: model.RemoteSourceID,
		RemoteSource:   model.RemoteSource,
		UpdatedAt:      model.UpdatedAt,
		CreatedAt:      model.CreatedAt,
	}

	if len(model.Translations) > 0 {
		dto.Translations = make(map[string]*dtos.PostTranslationContentDTO)
		for locale, translation := range model.Translations {
			dto.Translations[locale] = &dtos.PostTranslationContentDTO{
				Title:   translation.Title,
				Content: translation.Content,
			}
		}
	}

	if model.Metrics != nil {
		dto.Metrics = &dtos.PostMetricsDTO{
			PostID:    model.Metrics.PostID,
			Views:     model.Metrics.Views,
			Likes:     model.Metrics.Likes,
			Comments:  model.Metrics.Comments,
			UpdatedAt: model.Metrics.UpdatedAt,
			CreatedAt: model.Metrics.CreatedAt,
		}
	}

	return dto
}

func (c *PostConverter) ModelsToResponseDTOs(models []*models.Post) []dtos.PostResponseDTO {
	dtoList := make([]dtos.PostResponseDTO, len(models))
	for i, model := range models {
		dtoList[i] = c.ModelToResponseDTO(*model)
	}
	return dtoList
}

func (c *PostConverter) DTOTranslationToModel(dto *dtos.PostTranslationContentDTO) *models.PostTranslationContent {
	if dto == nil {
		return nil
	}
	return &models.PostTranslationContent{
		Title:   dto.Title,
		Content: dto.Content,
	}
}

func (c *PostConverter) DTOTranslationsToModel(dtoTranslations map[string]*dtos.PostTranslationContentDTO) map[string]*models.PostTranslationContent {
	if dtoTranslations == nil {
		return nil
	}

	modelTranslations := make(map[string]*models.PostTranslationContent)
	for locale, dto := range dtoTranslations {
		modelTranslations[locale] = c.DTOTranslationToModel(dto)
	}
	return modelTranslations
}
