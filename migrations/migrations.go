package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func GetMigrations() migrations.MigrationSource {
	builder := migrations.NewMigrationBuilder("gorest-blog")

	builder.Add(
		"20260102000003000",
		"create_posts_table",
		func(ctx context.Context, db database.Database) error {
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
					title TEXT NOT NULL,
					content TEXT NOT NULL,
					published_at TIMESTAMP(0) WITH TIME ZONE,
					updated_at TIMESTAMP(0) WITH TIME ZONE,
					created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				MySQL: `CREATE TABLE IF NOT EXISTS post (
					id CHAR(36) PRIMARY KEY,
					user_id CHAR(36),
					slug VARCHAR(255) NOT NULL,
					status ENUM('drafted', 'published') NOT NULL DEFAULT 'drafted',
					title TEXT NOT NULL,
					content TEXT NOT NULL,
					published_at TIMESTAMP NULL,
					updated_at TIMESTAMP NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
					INDEX idx_post_title (title(255)),
					INDEX idx_post_status (status),
					INDEX idx_post_fk_user (user_id),
					INDEX idx_post_slug (slug)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
				SQLite: `CREATE TABLE IF NOT EXISTS post (
					id TEXT PRIMARY KEY,
					user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
					slug TEXT NOT NULL,
					status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
					title TEXT NOT NULL,
					content TEXT NOT NULL,
					published_at TEXT,
					updated_at TEXT,
					created_at TEXT NOT NULL DEFAULT (datetime('now'))
				)`,
			}); err != nil {
				return err
			}

			// Create indexes for Postgres and SQLite
			if db.DriverName() == "postgres" {
				if err := migrations.CreateIndex(ctx, db, "idx_post_title", "post", "title"); err != nil {
					return err
				}
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
				if err := migrations.CreateIndex(ctx, db, "idx_post_title", "post", "title"); err != nil {
					return err
				}
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
		},
		func(ctx context.Context, db database.Database) error {
			// Drop indexes first
			if db.DriverName() == "postgres" || db.DriverName() == "sqlite" {
				_ = migrations.DropIndex(ctx, db, "idx_post_title", "post")
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
		},
	)

	builder.Add(
		"20260126000001000",
		"remove_post_title_content_columns",
		func(ctx context.Context, db database.Database) error {
			// Migrate existing post title/content to translatable table
			// Assumes default locale is 'en'
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `
					INSERT INTO translatable (id, user_id, translatable_id, translatable, locale, content, created_at)
					SELECT
						gen_random_uuid(),
						user_id,
						id,
						'post',
						'en',
						json_build_object('title', title, 'content', content)::text,
						created_at
					FROM post
					WHERE NOT EXISTS (
						SELECT 1 FROM translatable
						WHERE translatable_id = post.id
						AND translatable = 'post'
						AND locale = 'en'
					)
				`,
				MySQL: `
					INSERT INTO translatable (id, user_id, translatable_id, translatable, locale, content, created_at)
					SELECT
						UUID(),
						user_id,
						id,
						'post',
						'en',
						JSON_OBJECT('title', title, 'content', content),
						created_at
					FROM post
					WHERE NOT EXISTS (
						SELECT 1 FROM translatable
						WHERE translatable_id = post.id
						AND translatable = 'post'
						AND locale = 'en'
					)
				`,
				SQLite: `
					INSERT INTO translatable (id, user_id, translatable_id, translatable, locale, content, created_at)
					SELECT
						lower(hex(randomblob(16))),
						user_id,
						id,
						'post',
						'en',
						json_object('title', title, 'content', content),
						created_at
					FROM post
					WHERE NOT EXISTS (
						SELECT 1 FROM translatable
						WHERE translatable_id = post.id
						AND translatable = 'post'
						AND locale = 'en'
					)
				`,
			}); err != nil {
				return err
			}

			// Drop title and content columns
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE post DROP COLUMN IF EXISTS title, DROP COLUMN IF EXISTS content`,
				MySQL:    `ALTER TABLE post DROP COLUMN title, DROP COLUMN content`,
				SQLite:   `
					-- SQLite doesn't support DROP COLUMN before version 3.35.0
					-- Create new table without title and content
					CREATE TABLE post_new (
						id TEXT PRIMARY KEY,
						user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
						slug TEXT NOT NULL,
						status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
						published_at TEXT,
						updated_at TEXT,
						created_at TEXT NOT NULL DEFAULT (datetime('now'))
					);

					-- Copy data
					INSERT INTO post_new (id, user_id, slug, status, published_at, updated_at, created_at)
					SELECT id, user_id, slug, status, published_at, updated_at, created_at
					FROM post;

					-- Drop old table and rename new one
					DROP TABLE post;
					ALTER TABLE post_new RENAME TO post;
				`,
			}); err != nil {
				return err
			}

			// Recreate indexes for SQLite (they were dropped with the table)
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
		},
		func(ctx context.Context, db database.Database) error {
			// Add columns back
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `
					ALTER TABLE post ADD COLUMN IF NOT EXISTS title TEXT;
					ALTER TABLE post ADD COLUMN IF NOT EXISTS content TEXT;
				`,
				MySQL: `
					ALTER TABLE post
					ADD COLUMN title TEXT,
					ADD COLUMN content TEXT;
				`,
				SQLite: `
					-- SQLite doesn't support ADD COLUMN with NOT NULL and no DEFAULT
					-- Create new table with title and content
					CREATE TABLE post_new (
						id TEXT PRIMARY KEY,
						user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
						slug TEXT NOT NULL,
						status TEXT NOT NULL DEFAULT 'drafted' CHECK(status IN ('drafted', 'published')),
						title TEXT,
						content TEXT,
						published_at TEXT,
						updated_at TEXT,
						created_at TEXT NOT NULL DEFAULT (datetime('now'))
					);

					-- Copy data
					INSERT INTO post_new (id, user_id, slug, status, published_at, updated_at, created_at)
					SELECT id, user_id, slug, status, published_at, updated_at, created_at
					FROM post;

					-- Drop old table and rename new one
					DROP TABLE post;
					ALTER TABLE post_new RENAME TO post;
				`,
			}); err != nil {
				return err
			}

			// Migrate from translatable back to post (default locale only)
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `
					UPDATE post
					SET
						title = (
							SELECT content::jsonb->>'title'
							FROM translatable
							WHERE translatable_id = post.id
							AND translatable = 'post'
							AND locale = 'en'
							LIMIT 1
						),
						content = (
							SELECT content::jsonb->>'content'
							FROM translatable
							WHERE translatable_id = post.id
							AND translatable = 'post'
							AND locale = 'en'
							LIMIT 1
						)
				`,
				MySQL: `
					UPDATE post p
					LEFT JOIN translatable t ON t.translatable_id = p.id
						AND t.translatable = 'post'
						AND t.locale = 'en'
					SET
						p.title = JSON_UNQUOTE(JSON_EXTRACT(t.content, '$.title')),
						p.content = JSON_UNQUOTE(JSON_EXTRACT(t.content, '$.content'))
				`,
				SQLite: `
					UPDATE post
					SET
						title = (
							SELECT json_extract(content, '$.title')
							FROM translatable
							WHERE translatable_id = post.id
							AND translatable = 'post'
							AND locale = 'en'
							LIMIT 1
						),
						content = (
							SELECT json_extract(content, '$.content')
							FROM translatable
							WHERE translatable_id = post.id
							AND translatable = 'post'
							AND locale = 'en'
							LIMIT 1
						)
				`,
			}); err != nil {
				return err
			}

			// Recreate indexes for SQLite
			if db.DriverName() == "sqlite" {
				if err := migrations.CreateIndex(ctx, db, "idx_post_title", "post", "title"); err != nil {
					return err
				}
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
		},
	)

	return builder.Build()
}
