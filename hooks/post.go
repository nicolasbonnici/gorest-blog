package hooks

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/types"
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
	newArgs := make([]any, len(args))
	copy(newArgs, args)
	newArgs = append(newArgs, types.PostStatusPublished)

	placeholder := fmt.Sprintf("$%d", len(newArgs))
	statusCondition := fmt.Sprintf("status = %s", placeholder)

	// Use regex to find WHERE clause and insert AND condition before ORDER BY/LIMIT/OFFSET
	re := regexp.MustCompile(`(?i)^(.+\s+WHERE\s+.+?)(\s+(?:ORDER\s+BY|LIMIT|OFFSET)\s+.*)$`)

	if matches := re.FindStringSubmatch(query); matches != nil {
		return matches[1] + " AND " + statusCondition + matches[2], newArgs
	}

	if strings.Contains(strings.ToUpper(query), " WHERE ") {
		return query + " AND " + statusCondition, newArgs
	}

	re2 := regexp.MustCompile(`(?i)^(.+?)(\s+(?:ORDER\s+BY|LIMIT|OFFSET)\s+.*)$`)
	if matches := re2.FindStringSubmatch(query); matches != nil {
		return matches[1] + " WHERE " + statusCondition + matches[2], newArgs
	}

	return query + " WHERE " + statusCondition, newArgs
}
