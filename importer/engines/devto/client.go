package devto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://dev.to/api"
	DefaultTimeout = 30 * time.Second
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

// TagList is a custom type that can unmarshal both string and array from JSON
type TagList []string

func (t *TagList) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*t = TagList(arr)
		return nil
	}

	// If that fails, try as string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Parse comma-separated string
	if str != "" {
		parts := strings.Split(str, ",")
		tags := make([]string, 0, len(parts))
		for _, tag := range parts {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
		*t = TagList(tags)
	} else {
		*t = TagList([]string{})
	}

	return nil
}

type DevToArticle struct {
	ID              int       `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	BodyMarkdown    string    `json:"body_markdown"`
	URL             string    `json:"url"`
	PublishedAt     time.Time `json:"published_at"`
	EditedAt        time.Time `json:"edited_at"`
	CreatedAt       time.Time `json:"created_at"`
	TagList         TagList   `json:"tag_list"`
	Slug            string    `json:"slug"`
	CoverImage      string    `json:"cover_image"`
	CanonicalURL    string    `json:"canonical_url"`
	ReadingTimeMin  int       `json:"reading_time_minutes"`
	CommentsCount   int       `json:"comments_count"`
	PublicReactions int       `json:"public_reactions_count"`
	Archived        bool      `json:"archived"`
}

type DevToComment struct {
	ID        string             `json:"id_code"`
	CreatedAt time.Time          `json:"created_at"`
	BodyHTML  string             `json:"body_html"`
	Children  []DevToComment     `json:"children"`
	User      DevToCommentAuthor `json:"user,omitempty"`
}

type DevToCommentAuthor struct {
	Name            string `json:"name"`
	Username        string `json:"username"`
	ProfileImage    string `json:"profile_image"`
	ProfileImage90  string `json:"profile_image_90"`
	GithubUsername  string `json:"github_username"`
	TwitterUsername string `json:"twitter_username"`
	WebsiteURL      string `json:"website_url"`
}

func NewClient() *Client {
	return &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) GetArticlesByUsername(ctx context.Context, username string) ([]DevToArticle, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	endpoint := fmt.Sprintf("%s/articles", c.baseURL)

	params := url.Values{}
	params.Add("username", username)
	params.Add("per_page", "1000")

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	var articles []DevToArticle
	if err := c.doRequest(ctx, "GET", fullURL, &articles); err != nil {
		return nil, fmt.Errorf("failed to fetch articles for user %s: %w", username, err)
	}

	return articles, nil
}

func (c *Client) GetArticleByID(ctx context.Context, id int) (*DevToArticle, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid article ID: %d", id)
	}

	endpoint := fmt.Sprintf("%s/articles/%d", c.baseURL, id)

	var article DevToArticle
	if err := c.doRequest(ctx, "GET", endpoint, &article); err != nil {
		return nil, fmt.Errorf("failed to fetch article %d: %w", id, err)
	}

	return &article, nil
}

func (c *Client) GetArticleByURL(ctx context.Context, articleURL string) (*DevToArticle, error) {
	id, err := c.extractArticleIDFromURL(articleURL)
	if err != nil {
		return nil, err
	}

	return c.GetArticleByID(ctx, id)
}

func (c *Client) extractArticleIDFromURL(articleURL string) (int, error) {
	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	if !strings.Contains(parsedURL.Host, "dev.to") {
		return 0, fmt.Errorf("not a dev.to URL: %s", articleURL)
	}

	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid dev.to article URL format: %s", articleURL)
	}

	slug := parts[len(parts)-1]

	slugParts := strings.Split(slug, "-")
	if len(slugParts) == 0 {
		return 0, fmt.Errorf("could not extract ID from slug: %s", slug)
	}

	idStr := slugParts[len(slugParts)-1]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("could not parse article ID from URL: %w", err)
	}

	return id, nil
}

func (c *Client) GetCommentsByArticleID(ctx context.Context, articleID int) ([]DevToComment, error) {
	if articleID <= 0 {
		return nil, fmt.Errorf("invalid article ID: %d", articleID)
	}

	endpoint := fmt.Sprintf("%s/comments", c.baseURL)

	params := url.Values{}
	params.Add("a_id", fmt.Sprintf("%d", articleID))

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	var comments []DevToComment
	if err := c.doRequest(ctx, "GET", fullURL, &comments); err != nil {
		return nil, fmt.Errorf("failed to fetch comments for article %d: %w", articleID, err)
	}

	return comments, nil
}

type CreateArticleRequest struct {
	Article CreateArticlePayload `json:"article"`
}

type CreateArticlePayload struct {
	Title        string   `json:"title"`
	BodyMarkdown string   `json:"body_markdown"`
	Published    bool     `json:"published"`
	Tags         []string `json:"tags,omitempty"`
	CanonicalURL string   `json:"canonical_url,omitempty"`
}

type CreateArticleResponse struct {
	ID int `json:"id"`
}

func (c *Client) CreateArticle(ctx context.Context, apiKey string, payload CreateArticlePayload) (int, error) {
	if apiKey == "" {
		return 0, fmt.Errorf("API key is required for creating articles")
	}

	endpoint := fmt.Sprintf("%s/articles", c.baseURL)
	reqBody := CreateArticleRequest{Article: payload}

	var response CreateArticleResponse
	if err := c.doRequestWithBody(ctx, "POST", endpoint, apiKey, reqBody, &response); err != nil {
		return 0, fmt.Errorf("failed to create article: %w", err)
	}

	return response.ID, nil
}

func (c *Client) UpdateArticle(ctx context.Context, apiKey string, articleID int, payload CreateArticlePayload) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required for updating articles")
	}

	if articleID <= 0 {
		return fmt.Errorf("invalid article ID: %d", articleID)
	}

	endpoint := fmt.Sprintf("%s/articles/%d", c.baseURL, articleID)
	reqBody := CreateArticleRequest{Article: payload}

	var response DevToArticle
	if err := c.doRequestWithBody(ctx, "PUT", endpoint, apiKey, reqBody, &response); err != nil {
		return fmt.Errorf("failed to update article %d: %w", articleID, err)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GoREST-Blog-Importer/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (c *Client) doRequestWithBody(ctx context.Context, method, url, apiKey string, payload interface{}, result interface{}) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoREST-Blog-Importer/1.0")
	req.Header.Set("api-key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
