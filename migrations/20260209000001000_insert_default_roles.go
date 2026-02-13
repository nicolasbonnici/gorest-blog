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
			INSERT INTO roles (name, description, parent)
			VALUES
				('reader', 'Default authenticated user role with read access, can like and comment', NULL),
				('moderator', 'Can moderate content and manage posts', 'reader'),
				('writer', 'Can create, edit, and delete posts', 'moderator')
			ON CONFLICT (name) DO NOTHING;
		`,
		MySQL: `
			INSERT IGNORE INTO roles (name, description, parent)
			VALUES
				('reader', 'Default authenticated user role with read access, can like and comment', NULL),
				('moderator', 'Can moderate content and manage posts', 'reader'),
				('writer', 'Can create, edit, and delete posts', 'moderator');
		`,
		SQLite: `
			INSERT OR IGNORE INTO roles (name, description, parent)
			VALUES
				('reader', 'Default authenticated user role with read access, can like and comment', NULL),
				('moderator', 'Can moderate content and manage posts', 'reader'),
				('writer', 'Can create, edit, and delete posts', 'moderator');
		`,
	}); err != nil {
		return err
	}

	return nil
}

func insertDefaultRolesDown(ctx context.Context, db database.Database) error {
	// Remove default roles
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DELETE FROM roles WHERE name IN ('reader', 'moderator', 'writer');`,
		MySQL:    `DELETE FROM roles WHERE name IN ('reader', 'moderator', 'writer');`,
		SQLite:   `DELETE FROM roles WHERE name IN ('reader', 'moderator', 'writer');`,
	}); err != nil {
		return err
	}

	return nil
}
