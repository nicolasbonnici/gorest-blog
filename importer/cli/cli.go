package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nicolasbonnici/gorest/config"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/postgres"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"

	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest-blog/importer/engines"
	_ "github.com/nicolasbonnici/gorest-blog/importer/engines/devto"
)

type CLIProgressReporter struct {
	bar           *progressbar.ProgressBar
	lastMessage   string
	statusPrinted bool
	isTTY         bool
	writer        io.Writer
}

func (r *CLIProgressReporter) Start(total int, message string) {
	r.writer = os.Stdout
	r.isTTY = term.IsTerminal(int(os.Stdout.Fd()))

	fmt.Println(message)

	if r.isTTY {
		fmt.Println()
	}

	barOptions := []progressbar.Option{
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Progress[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionSetWriter(r.writer),
	}

	if r.isTTY {
		barOptions = append(barOptions, progressbar.OptionClearOnFinish())
	}

	r.bar = progressbar.NewOptions(total, barOptions...)
	r.statusPrinted = r.isTTY
}

func (r *CLIProgressReporter) Update(current int, message string) {
	if r.bar == nil {
		return
	}

	truncatedMsg := truncate(message, 80)

	if r.isTTY {
		if r.statusPrinted {
			fmt.Fprint(r.writer, "\033[1A")
		}

		fmt.Fprint(r.writer, "\033[2K\r")
		fmt.Fprintf(r.writer, "\033[36m→ %s\033[0m\n", truncatedMsg)
		r.statusPrinted = true
	} else {
		if message != r.lastMessage {
			fmt.Fprintf(r.writer, "→ %s\n", truncatedMsg)
		}
	}

	r.lastMessage = message
	_ = r.bar.Set(current)
}

func (r *CLIProgressReporter) Finish(message string) {
	if r.bar != nil {
		_ = r.bar.Finish()
	}

	if r.isTTY {
		if r.statusPrinted {
			fmt.Fprint(r.writer, "\033[1A\033[2K\r")
		}
	}

	fmt.Fprintln(r.writer)
	fmt.Println(message)
}

func (r *CLIProgressReporter) Error(err error) {
	fmt.Fprintf(os.Stderr, "[red]Error: %v[reset]\n", err)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func Run(args []string) int {
	fs := flag.NewFlagSet("import", flag.ExitOnError)

	source := fs.String("source", "devto", "Import engine to use")
	username := fs.String("username", "", "Username to import articles from")
	articleURL := fs.String("url", "", "Specific article URL to import")
	articleID := fs.String("id", "", "Specific article ID to import")
	userID := fs.String("user-id", "", "User ID to assign imported posts to (required)")
	truncate := fs.Bool("truncate", false, "Delete all existing posts before importing")
	dryRun := fs.Bool("dry-run", false, "Preview import without saving")
	importComments := fs.Bool("import-comments", true, "Import comments along with posts")
	commentsOnly := fs.Bool("comments-only", false, "Sync comments only, skip post create/update")
	listEngines := fs.Bool("list-engines", false, "List available engines")
	syncMode := fs.String("sync-mode", "local-wins", "Sync mode: local-wins (bidirectional), remote-wins (import only), import-only (new posts only)")
	apiKey := fs.String("api-key", "", "API key for remote source (required for pushing changes). Can also use DEVTO_API_KEY env var")
	forceUpdate := fs.Bool("force-update", false, "Force update all posts even if unchanged")
	configPath := fs.String("config", ".", "Path to gorest.yaml configuration file (default: current directory)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *listEngines {
		return handleListEngines()
	}

	if err := validateFlags(fs, *userID, *username, *articleURL, *articleID); err != nil {
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := setupDatabase(*configPath)
	if err != nil {
		return 1
	}
	defer func() { _ = db.Close() }()

	service, err := createImporterService(db)
	if err != nil {
		return 1
	}

	opts := buildImportOptions(*source, *userID, *username, *articleURL, *articleID, *truncate, *dryRun, *importComments, *commentsOnly, *syncMode, *apiKey, *forceUpdate)
	printImportMode(*dryRun, *truncate, opts.SyncMode, *forceUpdate, *commentsOnly)

	result, err := service.Import(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
		return 1
	}

	printImportSummary(result)

	if result.Failed > 0 {
		return 1
	}

	return 0
}

func handleListEngines() int {
	fmt.Println("Available import engines:")
	for _, name := range engines.List() {
		fmt.Printf("  - %s\n", name)
	}
	return 0
}

func validateFlags(fs *flag.FlagSet, userID, username, articleURL, articleID string) error {
	if userID == "" {
		fmt.Fprintln(os.Stderr, "Error: --user-id is required")
		fs.Usage()
		return fmt.Errorf("user-id required")
	}

	if username == "" && articleURL == "" && articleID == "" {
		fmt.Fprintln(os.Stderr, "Error: one of --username, --url, or --id must be provided")
		fs.Usage()
		return fmt.Errorf("source required")
	}

	return nil
}

func setupDatabase(configPath string) (database.Database, error) {
	// Load GoREST project configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load gorest.yaml: %v\n", err)
		return nil, err
	}

	if cfg.Database.URL == "" {
		fmt.Fprintln(os.Stderr, "Error: database.url not configured in gorest.yaml")
		return nil, fmt.Errorf("database.url not set")
	}

	// Determine database driver from URL
	driver := "postgres"
	if len(cfg.Database.URL) > 0 {
		switch {
		case cfg.Database.URL[:6] == "mysql:":
			driver = "mysql"
		case cfg.Database.URL[:7] == "sqlite:":
			driver = "sqlite"
		}
	}

	db, err := database.Open(driver, cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		return nil, err
	}

	return db, nil
}

func createImporterService(db database.Database) (importer.ImportService, error) {
	if importer.GetServiceFactory() == nil {
		fmt.Fprintln(os.Stderr, "Error: importer service factory not set - make sure to import blog package")
		return nil, fmt.Errorf("service factory not set")
	}

	reporter := &CLIProgressReporter{}
	service := importer.GetServiceFactory()(db, reporter)
	return service, nil
}

func buildImportOptions(source, userID, username, articleURL, articleID string, truncate, dryRun, importComments, commentsOnly bool, syncMode, apiKey string, forceUpdate bool) importer.ImportOptions {
	actualAPIKey := apiKey
	if actualAPIKey == "" {
		actualAPIKey = os.Getenv("DEVTO_API_KEY")
	}

	return importer.ImportOptions{
		Source:         source,
		UserID:         userID,
		Username:       username,
		ArticleURL:     articleURL,
		ArticleID:      articleID,
		Truncate:       truncate,
		DryRun:         dryRun,
		ImportComments: importComments,
		CommentsOnly:   commentsOnly,
		SyncMode:       importer.SyncMode(syncMode),
		APIKey:         actualAPIKey,
		ForceUpdate:    forceUpdate,
	}
}

func printImportMode(dryRun, truncate bool, syncMode importer.SyncMode, forceUpdate bool, commentsOnly bool) {
	if commentsOnly {
		fmt.Println("Comments-only mode: posts will not be created or updated")
		return
	}

	if dryRun {
		fmt.Println("Running in DRY-RUN mode - no changes will be saved")
	} else if truncate {
		fmt.Println("WARNING: All existing posts will be deleted before importing")
	}

	if forceUpdate {
		fmt.Println("Force update mode: All posts will be updated even if unchanged")
	}

	fmt.Printf("Sync mode: %s\n", syncMode)
	switch syncMode {
	case importer.SyncModeLocalWins:
		fmt.Println("  - Will import new remote posts and comments")
		fmt.Println("  - Will push local edits to remote")
		fmt.Println("  - Will create remote posts for local-only posts")
	case importer.SyncModeRemoteWins:
		fmt.Println("  - Will import/update posts and comments from remote")
		fmt.Println("  - Local changes will be overwritten")
	case importer.SyncModeImportOnly:
		fmt.Println("  - Will only import new posts")
		if !forceUpdate {
			fmt.Println("  - Existing posts will be skipped")
		}
	}
}

func printImportSummary(result *importer.ImportResult) {
	fmt.Println("\nImport Summary:")
	fmt.Printf("  Total fetched: %d\n", result.TotalFetched)
	fmt.Printf("  Created: %d\n", result.Created)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	fmt.Printf("  Failed: %d\n", result.Failed)
	if result.CommentsCreated > 0 {
		fmt.Printf("  Comments imported: %d\n", result.CommentsCreated)
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}
}
