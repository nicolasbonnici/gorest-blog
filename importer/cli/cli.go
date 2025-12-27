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

// CLIProgressReporter implements importer.ProgressReporter for CLI with progress bar
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

// Run executes the CLI logic and returns an exit code
// This is the main entry point for the importer CLI
func Run(args []string) int {
	fs := flag.NewFlagSet("import", flag.ExitOnError)

	source := fs.String("source", "devto", "Import engine to use")
	username := fs.String("username", "", "Username to import articles from")
	articleURL := fs.String("url", "", "Specific article URL to import")
	articleID := fs.String("id", "", "Specific article ID to import")
	userID := fs.String("user-id", "", "User ID to assign imported posts to (required)")
	update := fs.Bool("update", false, "Update existing posts with matching titles")
	dryRun := fs.Bool("dry-run", false, "Preview import without saving")
	listEngines := fs.Bool("list-engines", false, "List available engines")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	// List engines and exit
	if *listEngines {
		fmt.Println("Available import engines:")
		for _, name := range engines.List() {
			fmt.Printf("  - %s\n", name)
		}
		return 0
	}

	// Validate required flags
	if *userID == "" {
		fmt.Fprintln(os.Stderr, "Error: --user-id is required")
		fs.Usage()
		return 1
	}

	if *username == "" && *articleURL == "" && *articleID == "" {
		fmt.Fprintln(os.Stderr, "Error: one of --username, --url, or --id must be provided")
		fs.Usage()
		return 1
	}

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL environment variable is required")
		return 1
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Connect to database
	db, err := database.Open("postgres", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	// Create repository, reporter, and service
	repo := importer.NewRepository(db)
	reporter := &CLIProgressReporter{}
	service := importer.NewService(repo, reporter)

	// Build import options
	opts := importer.ImportOptions{
		Source:         *source,
		UserID:         *userID,
		Username:       *username,
		ArticleURL:     *articleURL,
		ArticleID:      *articleID,
		UpdateExisting: *update,
		DryRun:         *dryRun,
	}

	// Show dry-run notice
	if *dryRun {
		fmt.Println("Running in DRY-RUN mode - no changes will be saved")
	}

	// Execute import
	result, err := service.Import(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
		return 1
	}

	// Print summary
	fmt.Println("\nImport Summary:")
	fmt.Printf("  Total fetched: %d\n", result.TotalFetched)
	fmt.Printf("  Created: %d\n", result.Created)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	fmt.Printf("  Failed: %d\n", result.Failed)

	// Print errors if any
	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	// Return non-zero exit code if any imports failed
	if result.Failed > 0 {
		return 1
	}

	return 0
}
