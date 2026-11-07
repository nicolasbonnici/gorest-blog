package importer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/nicolasbonnici/gorest/database"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

type ImportRequest struct {
	Username       string `json:"username,omitempty"`
	ArticleURL     string `json:"url,omitempty"`
	ArticleID      string `json:"id,omitempty"`
	UserID         string `json:"user_id"`
	Truncate       bool   `json:"truncate,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
	ImportComments bool   `json:"import_comments,omitempty"`
	ForceUpdate    bool   `json:"force_update,omitempty"`
}

type ImportResponse struct {
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
	TotalFetched    int      `json:"total_fetched"`
	Created         int      `json:"created"`
	Updated         int      `json:"updated"`
	Skipped         int      `json:"skipped"`
	Failed          int      `json:"failed"`
	CommentsCreated int      `json:"comments_created,omitempty"`
	Errors          []string `json:"errors,omitempty"`
}

type EngineInfo struct {
	Name string `json:"name"`
}

type EnginesResponse struct {
	Engines []EngineInfo `json:"engines"`
}

type ServiceFactory func(db database.Database, reporter ProgressReporter) ImportService

type ImportService interface {
	Import(ctx context.Context, opts ImportOptions) (*ImportResult, error)
}

var serviceFactory ServiceFactory

func SetServiceFactory(factory ServiceFactory) {
	serviceFactory = factory
}

func GetServiceFactory() ServiceFactory {
	return serviceFactory
}

func executeImport(ctx context.Context, db database.Database, engine string, req ImportRequest) (*ImportResult, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	if req.Username == "" && req.ArticleURL == "" && req.ArticleID == "" {
		return nil, fmt.Errorf("one of username, url, or id must be provided")
	}

	if serviceFactory == nil {
		return nil, fmt.Errorf("service factory not set")
	}

	reporter := &NoOpProgressReporter{}
	service := serviceFactory(db, reporter)

	opts := ImportOptions{
		Source:         engine,
		UserID:         req.UserID,
		Username:       req.Username,
		ArticleURL:     req.ArticleURL,
		ArticleID:      req.ArticleID,
		Truncate:       req.Truncate,
		DryRun:         req.DryRun,
		ImportComments: req.ImportComments,
		ForceUpdate:    req.ForceUpdate,
	}

	result, err := service.Import(ctx, opts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func handleImport(db database.Database) fiber.Handler {
	return func(c fiber.Ctx) error {
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
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ImportResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
		defer cancel()

		result, err := executeImport(ctx, db, engine, req)
		if err != nil {
			// Log the detailed cause server-side; never echo internal state
			// (e.g. whether a given user_id exists) back to the client, as it
			// is an account-enumeration oracle and information leak.
			log.Printf("import failed (engine=%s): %v", engine, err)
			return c.Status(fiber.StatusBadRequest).JSON(ImportResponse{
				Success: false,
				Message: "import request could not be processed",
			})
		}

		errorMessages := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			errorMessages = append(errorMessages, err.Error())
		}

		return c.JSON(ImportResponse{
			Success:         result.Failed == 0,
			Message:         result.String(),
			TotalFetched:    result.TotalFetched,
			Created:         result.Created,
			Updated:         result.Updated,
			Skipped:         result.Skipped,
			Failed:          result.Failed,
			CommentsCreated: result.CommentsCreated,
			Errors:          errorMessages,
		})
	}
}

func handleListEngines() fiber.Handler {
	return func(c fiber.Ctx) error {
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

// RegisterRoutes mounts the import endpoints. The optional guards are applied
// as middleware ahead of every import handler; callers pass authentication and
// role checks here so the import surface is never anonymous. When no guards are
// supplied (for example when auth is disabled in a dev build) the routes are
// mounted bare.
func RegisterRoutes(router fiber.Router, db database.Database, guards ...fiber.Handler) {
	// Fiber runs the first handler, then the variadic ones, in order. Guards
	// must therefore run ahead of the final handler, so the first guard (if any)
	// becomes the lead handler and the rest trail it with the real handler last.
	importChain := chainFor(guards, handleImport(db))
	enginesChain := chainFor(guards, handleListEngines())

	router.Post("/api/import/:engine", importChain[0], importChain[1:]...)
	router.Get("/api/import/engines", enginesChain[0], enginesChain[1:]...)
}

// chainFor builds an ordered handler chain of the guards followed by the final
// handler, as a []any slice suitable for Fiber's variadic route registration.
func chainFor(guards []fiber.Handler, final fiber.Handler) []any {
	chain := make([]any, 0, len(guards)+1)
	for _, g := range guards {
		chain = append(chain, g)
	}
	chain = append(chain, final)
	return chain
}
