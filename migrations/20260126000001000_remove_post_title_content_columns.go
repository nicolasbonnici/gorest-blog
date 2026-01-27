package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260126000001000",
		"remove_post_title_content_columns",
		removePostTitleContentColumnsUp,
		removePostTitleContentColumnsDown,
	)
}

func removePostTitleContentColumnsUp(ctx context.Context, db database.Database) error {
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
		SQLite: `
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

func removePostTitleContentColumnsDown(ctx context.Context, db database.Database) error {
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
}
