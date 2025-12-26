package importer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest-blog-plugin/importer/engines"
	"github.com/nicolasbonnici/gorest-blog-plugin/types"
)

type Service struct {
	repository Repository
	reporter   ProgressReporter
}

func NewService(repo Repository, reporter ProgressReporter) *Service {
	if reporter == nil {
		reporter = &NoOpProgressReporter{}
	}
	return &Service{
		repository: repo,
		reporter:   reporter,
	}
}

func (s *Service) Import(ctx context.Context, opts ImportOptions) (*ImportResult, error) {
	engine, ok := engines.Get(opts.Source)
	if !ok {
		return nil, fmt.Errorf("unknown engine: %s (available: %v)", opts.Source, engines.List())
	}

	if opts.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Validate that the user exists before attempting to import
	userExists, err := s.repository.UserExists(ctx, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate user: %w", err)
	}
	if !userExists {
		return nil, fmt.Errorf("user_id '%s' does not exist", opts.UserID)
	}

	var posts []Post

	if opts.Username != "" {
		posts, err = engine.FetchByUsername(ctx, opts.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts by username: %w", err)
		}
	} else if opts.ArticleURL != "" {
		post, err := engine.FetchByURL(ctx, opts.ArticleURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by URL: %w", err)
		}
		posts = []Post{*post}
	} else if opts.ArticleID != "" {
		post, err := engine.FetchByID(ctx, opts.ArticleID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch post by ID: %w", err)
		}
		posts = []Post{*post}
	} else {
		return nil, fmt.Errorf("one of username, url, or id must be provided")
	}

	result := &ImportResult{
		TotalFetched: len(posts),
		Errors:       make([]error, 0),
	}

	if s.reporter != nil {
		s.reporter.Start(len(posts), fmt.Sprintf("Importing %d posts from %s", len(posts), opts.Source))
	}

	for i, post := range posts {
		if s.reporter != nil {
			s.reporter.Update(i+1, fmt.Sprintf("Processing: %s", post.Title))
		}

		action, err := s.importPost(ctx, post, opts, result)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to import '%s': %w", post.Title, err))
			if s.reporter != nil {
				s.reporter.Error(err)
			}
			continue
		}

		switch action {
		case "created":
			result.Created++
		case "updated":
			result.Updated++
		case "skipped":
			result.Skipped++
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	if s.reporter != nil {
		s.reporter.Finish(result.String())
	}

	return result, nil
}

func (s *Service) importPost(ctx context.Context, post Post, opts ImportOptions, result *ImportResult) (string, error) {
	postModel := s.postToModel(post, opts.UserID)

	if opts.DryRun {
		existing, err := s.repository.FindByTitle(ctx, post.Title)
		if err == nil && existing != nil {
			if opts.UpdateExisting {
				return "updated", nil
			}
			return "skipped", nil
		}
		return "created", nil
	}

	existing, err := s.repository.FindByTitle(ctx, post.Title)
	if err == nil && existing != nil {
		if opts.UpdateExisting {
			if err := s.repository.Update(ctx, existing.Id, &postModel); err != nil {
				return "", fmt.Errorf("update failed: %w", err)
			}
			return "updated", nil
		}
		return "skipped", nil
	}

	if err := s.repository.Create(ctx, &postModel); err != nil {
		return "", fmt.Errorf("create failed: %w", err)
	}

	return "created", nil
}

func (s *Service) postToModel(post Post, userID string) models.Post {
	// Determine status and parse published_at timestamp
	status := types.PostStatusDrafted
	var publishedAt *time.Time

	if post.PublishedAt != "" {
		// Try to parse the published date
		if parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", post.PublishedAt); err == nil {
			status = types.PostStatusPublished
			publishedAt = &parsedTime
		}
	}

	// Use slug from post, or generate from title if empty
	slug := post.Slug
	if slug == "" {
		slug = slugify(post.Title)
	}

	postModel := models.Post{
		Title:       post.Title,
		Content:     post.Content,
		Slug:        slug,
		Status:      string(status),
		PublishedAt: publishedAt,
		UserId:      &userID,
	}

	return postModel
}

// slugify converts a string into a URL-friendly slug
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove all non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	return s
}
