package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

// migrationRegistry stores all registered migrations
var migrationRegistry []migrationEntry

type migrationEntry struct {
	id   string
	name string
	up   func(context.Context, database.Database) error
	down func(context.Context, database.Database) error
}

func Register(id, name string, up, down func(context.Context, database.Database) error) {
	migrationRegistry = append(migrationRegistry, migrationEntry{
		id:   id,
		name: name,
		up:   up,
		down: down,
	})
}

func GetMigrations() migrations.MigrationSource {
	builder := migrations.NewMigrationBuilder("gorest-blog")

	for _, m := range migrationRegistry {
		builder.Add(m.id, m.name, m.up, m.down)
	}

	return builder.Build()
}
