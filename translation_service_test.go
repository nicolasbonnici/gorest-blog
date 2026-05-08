package blog

import (
	"strings"
	"testing"

	"github.com/nicolasbonnici/gorest-blog/models"
	"github.com/nicolasbonnici/gorest-blog/services"
)

func TestPostTranslationContent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content *models.PostTranslationContent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid content",
			content: &models.PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
			wantErr: false,
		},
		{
			name: "empty title",
			content: &models.PostTranslationContent{
				Title:   "",
				Content: "Test Content",
			},
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name: "whitespace only title",
			content: &models.PostTranslationContent{
				Title:   "   ",
				Content: "Test Content",
			},
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name: "empty content",
			content: &models.PostTranslationContent{
				Title:   "Test Title",
				Content: "",
			},
			wantErr: true,
			errMsg:  "content cannot be empty",
		},
		{
			name: "whitespace only content",
			content: &models.PostTranslationContent{
				Title:   "Test Title",
				Content: "   ",
			},
			wantErr: true,
			errMsg:  "content cannot be empty",
		},
		{
			name: "title with leading/trailing spaces",
			content: &models.PostTranslationContent{
				Title:   "  Test Title  ",
				Content: "Test Content",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestPostTranslationContent_Sanitize(t *testing.T) {
	tests := []struct {
		name            string
		input           *models.PostTranslationContent
		expectedTitle   string
		expectedContent string
	}{
		{
			name: "sanitize XSS in title only",
			input: &models.PostTranslationContent{
				Title:   "<script>alert('xss')</script>",
				Content: "Normal content",
			},
			expectedTitle:   "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
			expectedContent: "Normal content",
		},
		{
			name: "content not sanitized (markdown)",
			input: &models.PostTranslationContent{
				Title:   "Normal title",
				Content: "<img src=x onerror=alert('xss')>",
			},
			expectedTitle:   "Normal title",
			expectedContent: "<img src=x onerror=alert('xss')>",
		},
		{
			name: "only title HTML entities escaped",
			input: &models.PostTranslationContent{
				Title:   "Title & More",
				Content: "Content with <b>bold</b> & entities",
			},
			expectedTitle:   "Title &amp; More",
			expectedContent: "Content with <b>bold</b> & entities",
		},
		{
			name: "normal text unchanged",
			input: &models.PostTranslationContent{
				Title:   "Normal Title",
				Content: "Normal Content",
			},
			expectedTitle:   "Normal Title",
			expectedContent: "Normal Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.Sanitize()
			if tt.input.Title != tt.expectedTitle {
				t.Errorf("Sanitize() title = %v, want %v", tt.input.Title, tt.expectedTitle)
			}
			if tt.input.Content != tt.expectedContent {
				t.Errorf("Sanitize() content = %v, want %v", tt.input.Content, tt.expectedContent)
			}
		})
	}
}

func TestPostTranslationContent_ToJSON(t *testing.T) {
	tests := []struct {
		name    string
		content *models.PostTranslationContent
		wantErr bool
	}{
		{
			name: "serialize valid content",
			content: &models.PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
			wantErr: false,
		},
		{
			name: "serialize with special characters",
			content: &models.PostTranslationContent{
				Title:   "Title with \"quotes\" and \\ backslashes",
				Content: "Content with newlines\nand tabs\t",
			},
			wantErr: false,
		},
		{
			name: "serialize empty content",
			content: &models.PostTranslationContent{
				Title:   "",
				Content: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := services.ToJSON(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if jsonStr == "" {
					t.Errorf("ToJSON() returned empty string")
				}
				if !strings.Contains(jsonStr, "\"title\"") {
					t.Errorf("ToJSON() missing title field")
				}
				if !strings.Contains(jsonStr, "\"content\"") {
					t.Errorf("ToJSON() missing content field")
				}
			}
		})
	}
}

func TestParsePostTranslationContent(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "parse from string",
			input:   `{"title":"Test Title","content":"Test Content"}`,
			wantErr: false,
		},
		{
			name:    "parse from bytes",
			input:   []byte(`{"title":"Test Title","content":"Test Content"}`),
			wantErr: false,
		},
		{
			name:    "parse invalid JSON string",
			input:   `{"title":"Test Title","content":`,
			wantErr: true,
		},
		{
			name:    "parse invalid JSON bytes",
			input:   []byte(`{"title":"Test Title"`),
			wantErr: true,
		},
		{
			name:    "parse unsupported type",
			input:   12345,
			wantErr: true,
			errMsg:  "content must be []byte or string",
		},
		{
			name:    "parse empty JSON",
			input:   `{}`,
			wantErr: false,
		},
		{
			name:    "parse with escaped characters",
			input:   `{"title":"Title with \"quotes\"","content":"Content\nwith\nnewlines"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := services.ParsePostTranslationContent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("services.ParsePostTranslationContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("services.ParsePostTranslationContent() error = %v, want error containing %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && result == nil {
				t.Errorf("services.ParsePostTranslationContent() returned nil result without error")
			}
		})
	}
}

func TestPostTranslationContent_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		content *models.PostTranslationContent
	}{
		{
			name: "simple content",
			content: &models.PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
		},
		{
			name: "content with special characters",
			content: &models.PostTranslationContent{
				Title:   "Title with \"quotes\", newlines\n, and tabs\t",
				Content: "Content with special chars: & < > ' \"",
			},
		},
		{
			name: "unicode content",
			content: &models.PostTranslationContent{
				Title:   "Titre en français avec des accents: éàü",
				Content: "内容包含中文字符",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize to JSON
			jsonStr, err := services.ToJSON(tt.content)
			if err != nil {
				t.Fatalf("ToJSON() error = %v", err)
			}

			// Parse back from JSON
			parsed, err := services.ParsePostTranslationContent(jsonStr)
			if err != nil {
				t.Fatalf("services.ParsePostTranslationContent() error = %v", err)
			}

			// Compare
			if parsed.Title != tt.content.Title {
				t.Errorf("Round trip title mismatch: got %v, want %v", parsed.Title, tt.content.Title)
			}
			if parsed.Content != tt.content.Content {
				t.Errorf("Round trip content mismatch: got %v, want %v", parsed.Content, tt.content.Content)
			}
		})
	}
}

func TestTranslatableRecord_GetTranslationContent(t *testing.T) {
	tests := []struct {
		name    string
		record  *services.TranslatableRecord
		wantErr bool
	}{
		{
			name: "valid JSON content",
			record: &services.TranslatableRecord{
				Content: `{"title":"Test Title","content":"Test Content"}`,
			},
			wantErr: false,
		},
		{
			name: "invalid JSON content",
			record: &services.TranslatableRecord{
				Content: `{"title":"Test Title"`,
			},
			wantErr: true,
		},
		{
			name: "empty content",
			record: &services.TranslatableRecord{
				Content: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.record.GetTranslationContent()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTranslationContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Errorf("GetTranslationContent() returned nil result without error")
			}
		})
	}
}
