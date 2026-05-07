package devto

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nicolasbonnici/gorest-blog/importer/engines"
)

type Engine struct {
	client *Client
}

func NewEngine() *Engine {
	return &Engine{
		client: NewClient(),
	}
}

func (e *Engine) Name() string {
	return "devto"
}

func (e *Engine) FetchByUsername(ctx context.Context, username string) ([]engines.Post, error) {
	// First, get the list of articles (without full content)
	devtoArticles, err := e.client.GetArticlesByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch articles from dev.to: %w", err)
	}

	if len(devtoArticles) == 0 {
		return []engines.Post{}, nil
	}

	// Fetch full details for each article to get the body_markdown and comments
	posts := make([]engines.Post, 0, len(devtoArticles))
	for _, article := range devtoArticles {
		// Skip archived articles
		if article.Archived {
			fmt.Printf("Skipping archived article: %s (ID: %d)\n", article.Title, article.ID)
			continue
		}

		fullArticle, err := e.client.GetArticleByID(ctx, article.ID)
		if err != nil {
			// Log error but continue with other articles
			fmt.Printf("Warning: failed to fetch full details for article %d: %v\n", article.ID, err)
			continue
		}

		// Double-check archived status in full article (in case it changed)
		if fullArticle.Archived {
			fmt.Printf("Skipping archived article: %s (ID: %d)\n", fullArticle.Title, fullArticle.ID)
			continue
		}

		// Fetch comments for this article
		comments, err := e.client.GetCommentsByArticleID(ctx, article.ID)
		if err != nil {
			// Log error but continue without comments
			fmt.Printf("Warning: failed to fetch comments for article %d: %v\n", article.ID, err)
			comments = []DevToComment{}
		}

		posts = append(posts, MapPost(*fullArticle, comments))
	}

	return posts, nil
}

func (e *Engine) FetchByID(ctx context.Context, id string) (*engines.Post, error) {
	articleID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid article ID: %w", err)
	}

	devtoArticle, err := e.client.GetArticleByID(ctx, articleID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article from dev.to: %w", err)
	}

	// Skip archived articles
	if devtoArticle.Archived {
		return nil, fmt.Errorf("article %d is archived and cannot be imported", articleID)
	}

	// Fetch comments for this article
	comments, err := e.client.GetCommentsByArticleID(ctx, articleID)
	if err != nil {
		// Log error but continue without comments
		fmt.Printf("Warning: failed to fetch comments for article %d: %v\n", articleID, err)
		comments = []DevToComment{}
	}

	post := MapPost(*devtoArticle, comments)
	return &post, nil
}

func (e *Engine) FetchByURL(ctx context.Context, url string) (*engines.Post, error) {
	devtoArticle, err := e.client.GetArticleByURL(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article from dev.to: %w", err)
	}

	// Skip archived articles
	if devtoArticle.Archived {
		return nil, fmt.Errorf("article at %s is archived and cannot be imported", url)
	}

	// Extract article ID to fetch comments
	articleID := devtoArticle.ID
	comments, err := e.client.GetCommentsByArticleID(ctx, articleID)
	if err != nil {
		// Log error but continue without comments
		fmt.Printf("Warning: failed to fetch comments for article %d: %v\n", articleID, err)
		comments = []DevToComment{}
	}

	post := MapPost(*devtoArticle, comments)
	return &post, nil
}

func (e *Engine) CreatePost(ctx context.Context, apiKey string, post engines.Post) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("API key is required for creating posts on dev.to")
	}

	payload := CreateArticlePayload{
		Title:        post.Title,
		BodyMarkdown: post.Content,
		Published:    post.PublishedAt != "",
	}

	articleID, err := e.client.CreateArticle(ctx, apiKey, payload)
	if err != nil {
		return "", fmt.Errorf("failed to create article on dev.to: %w", err)
	}

	return fmt.Sprintf("%d", articleID), nil
}

func (e *Engine) UpdatePost(ctx context.Context, apiKey string, remoteID string, post engines.Post) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required for updating posts on dev.to")
	}

	articleID, err := strconv.Atoi(remoteID)
	if err != nil {
		return fmt.Errorf("invalid dev.to article ID %s: %w", remoteID, err)
	}

	payload := CreateArticlePayload{
		Title:        post.Title,
		BodyMarkdown: post.Content,
		Published:    post.PublishedAt != "",
	}

	if err := e.client.UpdateArticle(ctx, apiKey, articleID, payload); err != nil {
		return fmt.Errorf("failed to update article %s on dev.to: %w", remoteID, err)
	}

	return nil
}

func init() {
	engines.Register(NewEngine())
}
