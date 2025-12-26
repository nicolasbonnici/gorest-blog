package engines

import (
	"context"
)

// Engine defines the interface that all import engines must implement.
// An engine is responsible for fetching posts from a specific source
// (e.g., dev.to, Medium, HashNode) and converting them to the normalized Post format.
type Engine interface {
	// Name returns the unique identifier for this engine (e.g., "devto", "medium")
	Name() string

	// FetchByUsername fetches all posts for a given username on the source platform
	FetchByUsername(ctx context.Context, username string) ([]Post, error)

	// FetchByID fetches a single post by its ID on the source platform
	FetchByID(ctx context.Context, id string) (*Post, error)

	// FetchByURL fetches a single post from its URL on the source platform
	FetchByURL(ctx context.Context, url string) (*Post, error)
}
