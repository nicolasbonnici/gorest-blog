package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260102000003000",
		"create_posts_table",
		createPostsTableUp,
		createPostsTableDown,
	)
}

func createPostsTableUp(ctx context.Context, db database.Database) error {
	// Create post_status enum for Postgres
	if db.DriverName() == "postgres" {
		if err := migrations.SQL(ctx, db, migrations.DialectSQL{
			Postgres: `DO $$ BEGIN
				CREATE TYPE post_status AS ENUM ('drafted', 'published');
			EXCEPTION
				WHEN duplicate_object THEN null;
			END $$;`,
		}); err != nil {
			return err
		}
	}

	// Create posts table
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `CREATE TABLE IF NOT EXISTS post (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			slug TEXT NOT NULL,
			status post_status NOT NULL DEFAULT 'drafted',
			published_at TIMESTAMP(0) WITH TIME ZONE,
			updated_at TIMESTAMP(0) WITH TIME ZONE,
			created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		MySQL: `CREATE TABLE IF NOT EXISTS post (
			id CHAR(36) PRIMARY KEY,
			user_id CHAR(36),
			slug VARCHAR(255) NOT NULL,
			status ENUM('drafted', 'published') NOT NULL DEFAULT 'drafted',
			published_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
			INDEX idx_post_status (status),
			INDEX idx_post_fk_user (user_id),
			INDEX idx_post_slug (slug)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		SQLite: `CREATE TABLE IF NOT EXISTS post (
			id TEXT PRIMARY KEY,
			user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
			slug TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
			published_at TEXT,
			updated_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}); err != nil {
		return err
	}

	// Create indexes for Postgres and SQLite
	if db.DriverName() == "postgres" {
		if err := migrations.CreateIndex(ctx, db, "idx_post_status", "post", "status"); err != nil {
			return err
		}
		if err := migrations.CreateIndex(ctx, db, "idx_post_fk_user", "post", "user_id"); err != nil {
			return err
		}
		if err := migrations.CreateIndex(ctx, db, "idx_post_slug", "post", "slug"); err != nil {
			return err
		}
	}

	if db.DriverName() == "sqlite" {
		if err := migrations.CreateIndex(ctx, db, "idx_post_status", "post", "status"); err != nil {
			return err
		}
		if err := migrations.CreateIndex(ctx, db, "idx_post_fk_user", "post", "user_id"); err != nil {
			return err
		}
		if err := migrations.CreateIndex(ctx, db, "idx_post_slug", "post", "slug"); err != nil {
			return err
		}
	}

	return nil
}

func createPostsTableDown(ctx context.Context, db database.Database) error {
	// Drop indexes first
	if db.DriverName() == "postgres" || db.DriverName() == "sqlite" {
		_ = migrations.DropIndex(ctx, db, "idx_post_status", "post")
		_ = migrations.DropIndex(ctx, db, "idx_post_fk_user", "post")
		_ = migrations.DropIndex(ctx, db, "idx_post_slug", "post")
	}

	// Drop table
	if err := migrations.DropTableIfExists(ctx, db, "post"); err != nil {
		return err
	}

	// Drop enum for Postgres
	if db.DriverName() == "postgres" {
		_ = migrations.SQL(ctx, db, migrations.DialectSQL{
			Postgres: `DROP TYPE IF EXISTS post_status`,
		})
	}

	return nil
}
