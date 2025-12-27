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

	// Fetch full details for each article to get the body_markdown
	fullArticles := make([]DevToArticle, 0, len(devtoArticles))
	for _, article := range devtoArticles {
		fullArticle, err := e.client.GetArticleByID(ctx, article.ID)
		if err != nil {
			// Log error but continue with other articles
			fmt.Printf("Warning: failed to fetch full details for article %d: %v\n", article.ID, err)
			continue
		}
		fullArticles = append(fullArticles, *fullArticle)
	}

	posts := MapPosts(fullArticles)
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

	post := MapPost(*devtoArticle)
	return &post, nil
}

func (e *Engine) FetchByURL(ctx context.Context, url string) (*engines.Post, error) {
	devtoArticle, err := e.client.GetArticleByURL(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article from dev.to: %w", err)
	}

	post := MapPost(*devtoArticle)
	return &post, nil
}

func init() {
	engines.Register(NewEngine())
}
