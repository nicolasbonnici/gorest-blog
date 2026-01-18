package hooks

import (
	"context"
	"log"
	"time"

	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/types"
	"github.com/nicolasbonnici/gorest/hooks"
	"github.com/nicolasbonnici/gorest/query"
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
	return query, args, nil
}

func (h *PostHooks) AfterQuery(ctx context.Context, operation hooks.Operation, query string, args []any, result any, err error) error {
	return nil
}

func (h *PostHooks) OverrideQuery(ctx context.Context, operation hooks.Operation, id any, model *models.Post) (query string, args []any, skip bool) {
	return "", nil, false
}

func (h *PostHooks) ModifySelectQuery(ctx context.Context, operation hooks.Operation, builder *query.SelectBuilder) (*query.SelectBuilder, bool) {
	if operation == hooks.OperationGetAll || operation == hooks.OperationGetByID {
		if !isAuthenticated(ctx) {
			log.Printf("ModifySelectQuery: Adding status filter for unauthenticated user")
			builder = builder.Where(query.Eq("status", types.PostStatusPublished))
			return builder, true
		} else {
			log.Printf("ModifySelectQuery: Skipping status filter for authenticated user")
		}
	}
	return builder, false
}

func (h *PostHooks) ModifyUpdateQuery(ctx context.Context, operation hooks.Operation, id any, model *models.Post, builder *query.UpdateBuilder) (*query.UpdateBuilder, bool) {
	return builder, false
}

func (h *PostHooks) ModifyDeleteQuery(ctx context.Context, operation hooks.Operation, id any, builder *query.DeleteBuilder) (*query.DeleteBuilder, bool) {
	return builder, false
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
