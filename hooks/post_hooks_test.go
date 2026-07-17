package hooks

import (
	"context"
	"strings"
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

func TestNormalizeVisual(t *testing.T) {
	cases := []struct {
		name    string
		input   *string
		want    *string
		wantErr bool
	}{
		{"nil stays nil", nil, nil, false},
		{"valid https URL", ptr("https://cdn.example.com/a.png"), ptr("https://cdn.example.com/a.png"), false},
		{"valid http URL", ptr("http://example.com/a.png"), ptr("http://example.com/a.png"), false},
		{"surrounding whitespace trimmed", ptr("  https://example.com/a.png  "), ptr("https://example.com/a.png"), false},
		{"blank becomes nil", ptr("   "), nil, false},
		{"missing scheme", ptr("example.com/a.png"), nil, true},
		{"missing host", ptr("https://"), nil, true},
		{"unsupported scheme", ptr("javascript:alert(1)"), nil, true},
		{"too long", ptr("https://example.com/" + strings.Repeat("a", maxVisualLength)), nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			post := &models.Post{Visual: tc.input}

			err := normalizeVisual(post)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeVisual(%v) = nil error, want error", *tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeVisual returned unexpected error: %v", err)
			}

			switch {
			case tc.want == nil && post.Visual != nil:
				t.Fatalf("visual = %q, want nil", *post.Visual)
			case tc.want != nil && post.Visual == nil:
				t.Fatalf("visual = nil, want %q", *tc.want)
			case tc.want != nil && *post.Visual != *tc.want:
				t.Fatalf("visual = %q, want %q", *post.Visual, *tc.want)
			}
		})
	}
}

func ptr(s string) *string {
	return &s
}
