package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260302000001000",
		"add_remote_source_tracking",
		addRemoteSourceTrackingUp,
		addRemoteSourceTrackingDown,
	)
}

func addRemoteSourceTrackingUp(ctx context.Context, db database.Database) error {
	return migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
			ALTER TABLE post
			ADD COLUMN IF NOT EXISTS remote_source_id VARCHAR(255),
			ADD COLUMN IF NOT EXISTS remote_source VARCHAR(50)
		`,
		MySQL: `
			ALTER TABLE post
			ADD COLUMN remote_source_id VARCHAR(255),
			ADD COLUMN remote_source VARCHAR(50)
		`,
		SQLite: `
			ALTER TABLE post
			ADD COLUMN remote_source_id TEXT;
			ALTER TABLE post
			ADD COLUMN remote_source TEXT
		`,
	})
}

func addRemoteSourceTrackingDown(ctx context.Context, db database.Database) error {
	return migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
			ALTER TABLE post
			DROP COLUMN IF EXISTS remote_source_id,
			DROP COLUMN IF EXISTS remote_source
		`,
		MySQL: `
			ALTER TABLE post
			DROP COLUMN remote_source_id,
			DROP COLUMN remote_source
		`,
		SQLite: `
			-- SQLite doesn't support DROP COLUMN in older versions
			-- Would require table recreation, skipping for simplicity
			SELECT 1
		`,
	})
}
