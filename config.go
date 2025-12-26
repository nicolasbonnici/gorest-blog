package blog

import "github.com/nicolasbonnici/gorest/database"

type Config struct {
	Database         database.Database
	PaginationLimit  int
	MaxPaginationLimit int
	EnableImporter   bool
}

func DefaultConfig() Config {
	return Config{
		PaginationLimit:    10,
		MaxPaginationLimit: 1000,
		EnableImporter:     false,
	}
}
