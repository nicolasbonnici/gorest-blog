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
	// Check if title and content columns exist before migrating data
	hasColumns := false
	if db.DriverName() == "postgres" {
		var exists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='post' AND column_name='title'
			)
		`).Scan(&exists); err == nil && exists {
			hasColumns = true
		}
	} else if db.DriverName() == "mysql" {
		var exists bool
		if err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='post' AND column_name='title'
			)
		`).Scan(&exists); err == nil && exists {
			hasColumns = true
		}
	} else if db.DriverName() == "sqlite" {
		rows, err := db.Query(ctx, `PRAGMA table_info(post)`)
		if err == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var cid int
				var name string
				var typ string
				var notnull int
				var dfltValue interface{}
				var pk int
				if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err == nil {
					if name == "title" {
						hasColumns = true
						break
					}
				}
			}
		}
	}

	// Only migrate data if columns exist
	if hasColumns {
		if err := migrations.SQL(ctx, db, migrations.DialectSQL{
			Postgres: `
				INSERT INTO translations (id, user_id, translatable_id, translatable, locale, content, created_at)
				SELECT
					gen_random_uuid(),
					user_id,
					id,
					'post',
					'en',
					json_build_object('title', title, 'content', content),
					created_at
				FROM post
				WHERE NOT EXISTS (
					SELECT 1 FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
				)
			`,
			MySQL: `
				INSERT INTO translations (id, user_id, translatable_id, translatable, locale, content, created_at)
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
					SELECT 1 FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
				)
			`,
			SQLite: `
				INSERT INTO translations (id, user_id, translatable_id, translatable, locale, content, created_at)
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
					SELECT 1 FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
				)
			`,
		}); err != nil {
			return err
		}
	}

	// Drop title and content columns (if they exist)
	// Note: SQLite table recreation is handled separately below
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `ALTER TABLE post DROP COLUMN IF EXISTS title, DROP COLUMN IF EXISTS content`,
		MySQL: `
			ALTER TABLE post
			DROP COLUMN IF EXISTS title,
			DROP COLUMN IF EXISTS content
		`,
		SQLite: `SELECT 1`,
	}); err != nil {
		return err
	}

	// Handle SQLite table recreation if columns exist
	if db.DriverName() == "sqlite" && hasColumns {
		if err := migrations.SQL(ctx, db, migrations.DialectSQL{
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
					FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
					LIMIT 1
				),
				content = (
					SELECT content::jsonb->>'content'
					FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
					LIMIT 1
				)
		`,
		MySQL: `
			UPDATE post p
			LEFT JOIN translations t ON t.translatable_id = p.id
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
					FROM translations
					WHERE translatable_id = post.id
					AND translatable = 'post'
					AND locale = 'en'
					LIMIT 1
				),
				content = (
					SELECT json_extract(content, '$.content')
					FROM translations
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
