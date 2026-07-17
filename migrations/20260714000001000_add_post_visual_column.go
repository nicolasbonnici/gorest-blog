package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260714000001000",
		"add_post_visual_column",
		addPostVisualColumnUp,
		addPostVisualColumnDown,
	)
}

func addPostVisualColumnUp(ctx context.Context, db database.Database) error {
	return migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
			ALTER TABLE post
			ADD COLUMN IF NOT EXISTS visual VARCHAR(2048)
		`,
		MySQL: `
			ALTER TABLE post
			ADD COLUMN visual VARCHAR(2048)
		`,
		SQLite: `
			ALTER TABLE post
			ADD COLUMN visual TEXT
		`,
	})
}

func addPostVisualColumnDown(ctx context.Context, db database.Database) error {
	return migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
			ALTER TABLE post
			DROP COLUMN IF EXISTS visual
		`,
		MySQL: `
			ALTER TABLE post
			DROP COLUMN visual
		`,
		SQLite: `
			-- SQLite doesn't support DROP COLUMN in older versions
			-- Would require table recreation, skipping for simplicity
			SELECT 1
		`,
	})
}
