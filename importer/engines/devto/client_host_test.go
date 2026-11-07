package devto

import "testing"

func TestExtractArticleIDFromURL_HostValidation(t *testing.T) {
	c := &Client{}

	tests := []struct {
		name    string
		url     string
		wantID  int
		wantErr bool
	}{
		{"canonical dev.to", "https://dev.to/user/title-slug-123456", 123456, false},
		{"dev.to subdomain", "https://www.dev.to/user/title-slug-7890", 7890, false},
		{"spoofed suffix host rejected", "https://dev.to.evil.com/user/title-123", 0, true},
		{"unrelated host rejected", "https://evil.com/user/title-123", 0, true},
		{"host containing devdotto rejected", "https://notdev.to.attacker.net/a/b-1", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := c.extractArticleIDFromURL(tc.url)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got id=%d", tc.url, id)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.url, err)
			}
			if id != tc.wantID {
				t.Fatalf("for %q want id %d, got %d", tc.url, tc.wantID, id)
			}
		})
	}
}
