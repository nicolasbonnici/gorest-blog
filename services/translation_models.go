package services

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest-blog/models"
)

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
func (t *TranslatableRecord) GetTranslationContent() (*models.PostTranslationContent, error) {
	return ParsePostTranslationContent(t.Content)
}

// ToJSON serializes the PostTranslationContent to JSON string
// This is used for database storage in all database types
func ToJSON(p *models.PostTranslationContent) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParsePostTranslationContent deserializes from database JSON
// Handles both []byte (from JSON columns) and string (from TEXT)
func ParsePostTranslationContent(content interface{}) (*models.PostTranslationContent, error) {
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

	var ptc models.PostTranslationContent
	if err = json.Unmarshal(data, &ptc); err != nil {
		return nil, err
	}

	return &ptc, nil
}
