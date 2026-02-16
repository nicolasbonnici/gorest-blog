package blog

import (
	"strings"
	"testing"
)

func TestPostTranslationContent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content *PostTranslationContent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid content",
			content: &PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
			wantErr: false,
		},
		{
			name: "empty title",
			content: &PostTranslationContent{
				Title:   "",
				Content: "Test Content",
			},
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name: "whitespace only title",
			content: &PostTranslationContent{
				Title:   "   ",
				Content: "Test Content",
			},
			wantErr: true,
			errMsg:  "title cannot be empty",
		},
		{
			name: "empty content",
			content: &PostTranslationContent{
				Title:   "Test Title",
				Content: "",
			},
			wantErr: true,
			errMsg:  "content cannot be empty",
		},
		{
			name: "whitespace only content",
			content: &PostTranslationContent{
				Title:   "Test Title",
				Content: "   ",
			},
			wantErr: true,
			errMsg:  "content cannot be empty",
		},
		{
			name: "title with leading/trailing spaces",
			content: &PostTranslationContent{
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
		input           *PostTranslationContent
		expectedTitle   string
		expectedContent string
	}{
		{
			name: "sanitize XSS in title",
			input: &PostTranslationContent{
				Title:   "<script>alert('xss')</script>",
				Content: "Normal content",
			},
			expectedTitle:   "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
			expectedContent: "Normal content",
		},
		{
			name: "sanitize XSS in content",
			input: &PostTranslationContent{
				Title:   "Normal title",
				Content: "<img src=x onerror=alert('xss')>",
			},
			expectedTitle:   "Normal title",
			expectedContent: "&lt;img src=x onerror=alert(&#39;xss&#39;)&gt;",
		},
		{
			name: "sanitize HTML entities",
			input: &PostTranslationContent{
				Title:   "Title & More",
				Content: "Content with <b>bold</b> & entities",
			},
			expectedTitle:   "Title &amp; More",
			expectedContent: "Content with &lt;b&gt;bold&lt;/b&gt; &amp; entities",
		},
		{
			name: "normal text unchanged",
			input: &PostTranslationContent{
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
		content *PostTranslationContent
		wantErr bool
	}{
		{
			name: "serialize valid content",
			content: &PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
			wantErr: false,
		},
		{
			name: "serialize with special characters",
			content: &PostTranslationContent{
				Title:   "Title with \"quotes\" and \\ backslashes",
				Content: "Content with newlines\nand tabs\t",
			},
			wantErr: false,
		},
		{
			name: "serialize empty content",
			content: &PostTranslationContent{
				Title:   "",
				Content: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := tt.content.ToJSON()
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
			result, err := ParsePostTranslationContent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePostTranslationContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ParsePostTranslationContent() error = %v, want error containing %v", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && result == nil {
				t.Errorf("ParsePostTranslationContent() returned nil result without error")
			}
		})
	}
}

func TestPostTranslationContent_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		content *PostTranslationContent
	}{
		{
			name: "simple content",
			content: &PostTranslationContent{
				Title:   "Test Title",
				Content: "Test Content",
			},
		},
		{
			name: "content with special characters",
			content: &PostTranslationContent{
				Title:   "Title with \"quotes\", newlines\n, and tabs\t",
				Content: "Content with special chars: & < > ' \"",
			},
		},
		{
			name: "unicode content",
			content: &PostTranslationContent{
				Title:   "Titre en français avec des accents: éàü",
				Content: "内容包含中文字符",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize to JSON
			jsonStr, err := tt.content.ToJSON()
			if err != nil {
				t.Fatalf("ToJSON() error = %v", err)
			}

			// Parse back from JSON
			parsed, err := ParsePostTranslationContent(jsonStr)
			if err != nil {
				t.Fatalf("ParsePostTranslationContent() error = %v", err)
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
		record  *TranslatableRecord
		wantErr bool
	}{
		{
			name: "valid JSON content",
			record: &TranslatableRecord{
				Content: `{"title":"Test Title","content":"Test Content"}`,
			},
			wantErr: false,
		},
		{
			name: "invalid JSON content",
			record: &TranslatableRecord{
				Content: `{"title":"Test Title"`,
			},
			wantErr: true,
		},
		{
			name: "empty content",
			record: &TranslatableRecord{
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
