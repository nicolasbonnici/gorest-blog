package importer

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-blog-plugin/importer/engines"
	"github.com/nicolasbonnici/gorest/database"
)

type ImportRequest struct {
	Username       string `json:"username,omitempty"`
	ArticleURL     string `json:"url,omitempty"`
	ArticleID      string `json:"id,omitempty"`
	UserID         string `json:"user_id"`
	UpdateExisting bool   `json:"update_existing,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
}

type ImportResponse struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	TotalFetched int      `json:"total_fetched"`
	Created      int      `json:"created"`
	Updated      int      `json:"updated"`
	Skipped      int      `json:"skipped"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors,omitempty"`
}

type EngineInfo struct {
	Name string `json:"name"`
}

type EnginesResponse struct {
	Engines []EngineInfo `json:"engines"`
}

func executeImport(ctx context.Context, db database.Database, engine string, req ImportRequest) (*ImportResult, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	if req.Username == "" && req.ArticleURL == "" && req.ArticleID == "" {
		return nil, fmt.Errorf("one of username, url, or id must be provided")
	}

	repo := NewRepository(db)
	reporter := &NoOpProgressReporter{}
	service := NewService(repo, reporter)

	opts := ImportOptions{
		Source:         engine,
		UserID:         req.UserID,
		Username:       req.Username,
		ArticleURL:     req.ArticleURL,
		ArticleID:      req.ArticleID,
		UpdateExisting: req.UpdateExisting,
		DryRun:         req.DryRun,
	}

	result, err := service.Import(ctx, opts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func handleImport(db database.Database) fiber.Handler {
	return func(c *fiber.Ctx) error {
		engine := c.Params("engine")
		if engine == "" {
			return c.Status(fiber.StatusBadRequest).JSON(ImportResponse{
				Success: false,
				Message: "engine parameter is required",
			})
		}

		if _, ok := engines.Get(engine); !ok {
			return c.Status(fiber.StatusBadRequest).JSON(ImportResponse{
				Success: false,
				Message: fmt.Sprintf("unknown engine: %s (available: %v)", engine, engines.List()),
			})
		}

		var req ImportRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ImportResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
		defer cancel()

		result, err := executeImport(ctx, db, engine, req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ImportResponse{
				Success: false,
				Message: fmt.Sprintf("Import failed: %v", err),
			})
		}

		errorMessages := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			errorMessages = append(errorMessages, err.Error())
		}

		return c.JSON(ImportResponse{
			Success:      result.Failed == 0,
			Message:      result.String(),
			TotalFetched: result.TotalFetched,
			Created:      result.Created,
			Updated:      result.Updated,
			Skipped:      result.Skipped,
			Failed:       result.Failed,
			Errors:       errorMessages,
		})
	}
}

func handleListEngines() fiber.Handler {
	return func(c *fiber.Ctx) error {
		engineNames := engines.List()
		engineInfos := make([]EngineInfo, 0, len(engineNames))
		for _, name := range engineNames {
			engineInfos = append(engineInfos, EngineInfo{Name: name})
		}

		return c.JSON(EnginesResponse{
			Engines: engineInfos,
		})
	}
}

func RegisterRoutes(router fiber.Router, db database.Database) {
	router.Post("/api/import/:engine", handleImport(db))
	router.Get("/api/import/engines", handleListEngines())
}
