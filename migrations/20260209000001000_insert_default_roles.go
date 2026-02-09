package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260209000001000",
		"insert_default_roles",
		insertDefaultRolesUp,
		insertDefaultRolesDown,
	)
}

func insertDefaultRolesUp(ctx context.Context, db database.Database) error {
	// Insert default roles: reader, moderator, writer
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
			INSERT INTO roles (id, name, description, parent, created_at)
			VALUES
				('reader', 'reader', 'Default authenticated user role with read access, can like and comment', NULL, CURRENT_TIMESTAMP),
				('moderator', 'moderator', 'Can moderate content and manage posts', 'reader', CURRENT_TIMESTAMP),
				('writer', 'writer', 'Can create, edit, and delete posts', 'moderator', CURRENT_TIMESTAMP)
			ON CONFLICT (id) DO NOTHING;
		`,
		MySQL: `
			INSERT IGNORE INTO roles (id, name, description, parent, created_at)
			VALUES
				('reader', 'reader', 'Default authenticated user role with read access, can like and comment', NULL, NOW()),
				('moderator', 'moderator', 'Can moderate content and manage posts', 'reader', NOW()),
				('writer', 'writer', 'Can create, edit, and delete posts', 'moderator', NOW());
		`,
		SQLite: `
			INSERT OR IGNORE INTO roles (id, name, description, parent, created_at)
			VALUES
				('reader', 'reader', 'Default authenticated user role with read access, can like and comment', NULL, datetime('now')),
				('moderator', 'moderator', 'Can moderate content and manage posts', 'reader', datetime('now')),
				('writer', 'writer', 'Can create, edit, and delete posts', 'moderator', datetime('now'));
		`,
	}); err != nil {
		return err
	}

	return nil
}

func insertDefaultRolesDown(ctx context.Context, db database.Database) error {
	// Remove default roles
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DELETE FROM roles WHERE id IN ('reader', 'moderator', 'writer');`,
		MySQL:    `DELETE FROM roles WHERE id IN ('reader', 'moderator', 'writer');`,
		SQLite:   `DELETE FROM roles WHERE id IN ('reader', 'moderator', 'writer');`,
	}); err != nil {
		return err
	}

	return nil
}
