package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nicolasbonnici/gorest-blog/importer"
	"github.com/nicolasbonnici/gorest-blog/importer/engines"
	_ "github.com/nicolasbonnici/gorest-blog/importer/engines/devto"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/postgres"
	"github.com/schollz/progressbar/v3"
)

type CLIProgressReporter struct {
	bar *progressbar.ProgressBar
}

func (r *CLIProgressReporter) Start(total int, message string) {
	fmt.Println(message)
	r.bar = progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Importing...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func (r *CLIProgressReporter) Update(current int, message string) {
	if r.bar != nil {
		r.bar.Describe(fmt.Sprintf("[cyan]%s[reset]", truncate(message, 60)))
		_ = r.bar.Set(current)
	}
}

func (r *CLIProgressReporter) Finish(message string) {
	if r.bar != nil {
		_ = r.bar.Finish()
	}
	fmt.Println("\n" + message)
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
	importComments := fs.Bool("import-comments", false, "Import comments along with posts")
	listEngines := fs.Bool("list-engines", false, "List available engines")

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

	db, err := setupDatabase()
	if err != nil {
		return 1
	}
	defer func() { _ = db.Close() }()

	service, err := createImporterService(db)
	if err != nil {
		return 1
	}

	opts := buildImportOptions(*source, *userID, *username, *articleURL, *articleID, *truncate, *dryRun, *importComments)
	printImportMode(*dryRun, *truncate)

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

func setupDatabase() (database.Database, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL environment variable is required")
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	db, err := database.Open("postgres", databaseURL)
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

func buildImportOptions(source, userID, username, articleURL, articleID string, truncate, dryRun, importComments bool) importer.ImportOptions {
	return importer.ImportOptions{
		Source:         source,
		UserID:         userID,
		Username:       username,
		ArticleURL:     articleURL,
		ArticleID:      articleID,
		Truncate:       truncate,
		DryRun:         dryRun,
		ImportComments: importComments,
	}
}

func printImportMode(dryRun, truncate bool) {
	if dryRun {
		fmt.Println("Running in DRY-RUN mode - no changes will be saved")
	} else if truncate {
		fmt.Println("WARNING: All existing posts will be deleted before importing")
	}
}

func printImportSummary(result *importer.ImportResult) {
	fmt.Println("\nImport Summary:")
	fmt.Printf("  Total fetched: %d\n", result.TotalFetched)
	fmt.Printf("  Created: %d\n", result.Created)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	fmt.Printf("  Failed: %d\n", result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}
}
