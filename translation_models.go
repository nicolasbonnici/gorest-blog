package blog

import (
	"encoding/json"
	"errors"
	"html"
	"strings"

	"github.com/google/uuid"
)

// PostTranslationContent represents the JSON structure stored in translations.content
// This structure contains both title and content fields for a post translation
type PostTranslationContent struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Validate validates the PostTranslationContent fields
func (p *PostTranslationContent) Validate() error {
	p.Title = strings.TrimSpace(p.Title)
	p.Content = strings.TrimSpace(p.Content)

	if p.Title == "" {
		return errors.New("title cannot be empty")
	}

	if p.Content == "" {
		return errors.New("content cannot be empty")
	}

	return nil
}

// Sanitize applies HTML escaping to prevent XSS attacks
func (p *PostTranslationContent) Sanitize() {
	p.Title = html.EscapeString(p.Title)
	p.Content = html.EscapeString(p.Content)
}

// ToJSON serializes the PostTranslationContent to JSON string
// This is used for database storage in all database types
func (p *PostTranslationContent) ToJSON() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParsePostTranslationContent deserializes from database JSON
// Handles both []byte (from JSON columns) and string (from TEXT)
func ParsePostTranslationContent(content interface{}) (*PostTranslationContent, error) {
	var data []byte
	var err error

	switch v := content.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return nil, errors.New("content must be []byte or string")
	}

	var ptc PostTranslationContent
	if err = json.Unmarshal(data, &ptc); err != nil {
		return nil, err
	}

	return &ptc, nil
}

// TranslatableRecord represents a record from the translatable table
// This is used internally by the TranslationService
type TranslatableRecord struct {
	ID             uuid.UUID  `db:"id"`
	UserID         *uuid.UUID `db:"user_id"`
	TranslatableID uuid.UUID  `db:"translatable_id"`
	Translatable   string     `db:"translatable"`
	Locale         string     `db:"locale"`
	Content        string     `db:"content"`
	UpdatedAt      *string    `db:"updated_at"`
	CreatedAt      string     `db:"created_at"`
}

// GetTranslationContent parses the content field into PostTranslationContent
func (t *TranslatableRecord) GetTranslationContent() (*PostTranslationContent, error) {
	return ParsePostTranslationContent(t.Content)
}
