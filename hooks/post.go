package hooks

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog-plugin/models"
	"github.com/nicolasbonnici/gorest-blog-plugin/types"
	"github.com/nicolasbonnici/gorest/hooks"
)

type PostHooks struct{}

func (h *PostHooks) StateProcessor(ctx context.Context, operation hooks.Operation, id any, post *models.Post) error {
	if operation == hooks.OperationCreate {
		if userID := ctx.Value("user_id"); userID != nil {
			if uid, ok := userID.(string); ok {
				post.UserId = &uid
				log.Printf("StateProcessor: Set userId to %s", uid)
			}
		}

		if post.Status == "" {
			post.Status = string(types.PostStatusDrafted)
			log.Printf("StateProcessor: Set default status to '%s'", types.PostStatusDrafted)
		}
	}

	if operation == hooks.OperationCreate || operation == hooks.OperationUpdate {
		if post.Status == string(types.PostStatusPublished) && post.PublishedAt == nil {
			now := time.Now()
			post.PublishedAt = &now
			log.Printf("StateProcessor: Set publishedAt to %s for post being published", now.Format(time.RFC3339))
		}
	}

	return nil
}

func (h *PostHooks) BeforeQuery(ctx context.Context, operation hooks.Operation, query string, args []any) (string, []any, error) {
	log.Printf("[PostHooks] BeforeQuery called - operation: %s, query: %s", operation, query)

	if operation == hooks.OperationGetAll || operation == hooks.OperationGetByID {
		if !isAuthenticated(ctx) {
			modifiedQuery, modifiedArgs := addStatusFilter(query, args)
			log.Printf("BeforeQuery: Added status filter for unauthenticated user")
			log.Printf("Original query: %s", query)
			log.Printf("Modified query: %s", modifiedQuery)
			return modifiedQuery, modifiedArgs, nil
		} else {
			log.Printf("BeforeQuery: Skipping status filter for authenticated user")
		}
	}
	return query, args, nil
}

func (h *PostHooks) AfterQuery(ctx context.Context, operation hooks.Operation, query string, args []any, result any, err error) error {
	return nil
}

func (h *PostHooks) OverrideQuery(ctx context.Context, operation hooks.Operation, id any, model *models.Post) (query string, args []any, skip bool) {
	return "", nil, false
}

func (h *PostHooks) SerializeOne(ctx context.Context, operation hooks.Operation, post *models.Post) error {
	if post.UserId != nil {
		log.Printf("SerializeOne: Post %s has userId: %s", post.Id, *post.UserId)
	} else {
		log.Printf("SerializeOne: Post %s has NIL userId", post.Id)
	}
	return nil
}

func (h *PostHooks) SerializeMany(ctx context.Context, operation hooks.Operation, posts *[]models.Post) error {
	for i := range *posts {
		_ = h.SerializeOne(ctx, operation, &(*posts)[i])
	}
	return nil
}

func isAuthenticated(ctx context.Context) bool {
	if userID := ctx.Value("user_id"); userID != nil {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			log.Printf("[Auth] User authenticated: %s", userIDStr)
			return true
		}
	}
	log.Printf("[Auth] User not authenticated")
	return false
}

func addStatusFilter(query string, args []any) (string, []any) {
	queryLower := strings.ToLower(query)

	newArgs := make([]any, len(args))
	copy(newArgs, args)
	newArgs = append(newArgs, types.PostStatusPublished)

	placeholder := fmt.Sprintf("$%d", len(newArgs))

	if strings.Contains(queryLower, " where ") {
		parts := strings.SplitN(query, " WHERE ", 2)
		if len(parts) == 2 {
			return parts[0] + " WHERE " + parts[1] + fmt.Sprintf(" AND status = %s", placeholder), newArgs
		}
		parts = strings.SplitN(query, " where ", 2)
		if len(parts) == 2 {
			return parts[0] + " where " + parts[1] + fmt.Sprintf(" AND status = %s", placeholder), newArgs
		}
	}

	if strings.Contains(queryLower, " order by ") {
		parts := strings.SplitN(query, " ORDER BY ", 2)
		if len(parts) == 2 {
			return parts[0] + fmt.Sprintf(" WHERE status = %s ORDER BY ", placeholder) + parts[1], newArgs
		}
		parts = strings.SplitN(query, " order by ", 2)
		if len(parts) == 2 {
			return parts[0] + fmt.Sprintf(" WHERE status = %s order by ", placeholder) + parts[1], newArgs
		}
	}

	if strings.Contains(queryLower, " limit ") {
		parts := strings.SplitN(query, " LIMIT ", 2)
		if len(parts) == 2 {
			return parts[0] + fmt.Sprintf(" WHERE status = %s LIMIT ", placeholder) + parts[1], newArgs
		}
		parts = strings.SplitN(query, " limit ", 2)
		if len(parts) == 2 {
			return parts[0] + fmt.Sprintf(" WHERE status = %s limit ", placeholder) + parts[1], newArgs
		}
	}

	return query + fmt.Sprintf(" WHERE status = %s", placeholder), newArgs
}
