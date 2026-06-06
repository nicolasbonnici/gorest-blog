package hooks

import (
	"context"
	"testing"
	"time"

	"github.com/nicolasbonnici/gorest/rbac"

	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/types"
)

func TestIsPrivilegedReader(t *testing.T) {
	h := &PostHooks{}

	cases := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"no roles", nil, false},
		{"reader only", []string{"reader"}, false},
		{"writer", []string{"writer"}, true},
		{"moderator", []string{"moderator"}, true},
		{"admin", []string{"admin"}, true},
		{"mixed with privileged", []string{"reader", "writer"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.roles != nil {
				ctx = rbac.WithRoles(ctx, tc.roles)
			}
			if got := h.isPrivilegedReader(ctx); got != tc.want {
				t.Fatalf("isPrivilegedReader(%v) = %v, want %v", tc.roles, got, tc.want)
			}
		})
	}
}

func TestCheckReadAccess_privilegedSeesDraft(t *testing.T) {
	h := &PostHooks{}
	ctx := rbac.WithRoles(context.Background(), []string{"writer"})

	post := &models.Post{Status: types.PostStatusDrafted}
	if err := h.checkReadAccessForCtx(ctx, post); err != nil {
		t.Fatalf("privileged reader should access draft, got %v", err)
	}
}

func TestCheckReadAccess_anonymousBlockedFromDraft(t *testing.T) {
	h := &PostHooks{}
	ctx := context.Background()

	post := &models.Post{Status: types.PostStatusDrafted}
	if err := h.checkReadAccessForCtx(ctx, post); err == nil {
		t.Fatal("anonymous should get error on draft")
	}
}

func TestCheckReadAccess_anonymousBlockedFromScheduled(t *testing.T) {
	h := &PostHooks{}
	ctx := context.Background()

	future := time.Now().Add(24 * time.Hour)
	post := &models.Post{Status: types.PostStatusPublished, PublishedAt: &future}
	if err := h.checkReadAccessForCtx(ctx, post); err == nil {
		t.Fatal("anonymous should get error on scheduled (future) post")
	}
}

func TestCheckReadAccess_anonymousBlockedFromPublishedWithoutDate(t *testing.T) {
	h := &PostHooks{}
	ctx := context.Background()

	post := &models.Post{Status: types.PostStatusPublished, PublishedAt: nil}
	if err := h.checkReadAccessForCtx(ctx, post); err == nil {
		t.Fatal("published with nil published_at should be hidden from anonymous")
	}
}

func TestCheckReadAccess_anonymousSeesPublished(t *testing.T) {
	h := &PostHooks{}
	ctx := context.Background()

	past := time.Now().Add(-time.Hour)
	post := &models.Post{Status: types.PostStatusPublished, PublishedAt: &past}
	if err := h.checkReadAccessForCtx(ctx, post); err != nil {
		t.Fatalf("anonymous should read published past post, got %v", err)
	}
}
