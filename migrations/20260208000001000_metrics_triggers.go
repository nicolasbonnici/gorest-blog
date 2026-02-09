package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func init() {
	Register(
		"20260208000001000",
		"metrics_triggers",
		metricTriggersUp,
		metricTriggersDown,
	)
}

func metricTriggersUp(ctx context.Context, db database.Database) error {
	// Only create triggers for PostgreSQL
	if db.DriverName() != "postgres" {
		return nil
	}

	// Create function to increment metrics
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION increment_metric(
	p_resource TEXT,
	p_resource_id UUID,
	p_name TEXT
) RETURNS VOID AS $$
BEGIN
	INSERT INTO metrics (resource, resource_id, name, value)
	VALUES (p_resource, p_resource_id, p_name, 1)
	ON CONFLICT (resource, resource_id, name)
	DO UPDATE SET value = metrics.value + 1;
END;
$$ LANGUAGE plpgsql;
`,
	}); err != nil {
		return err
	}

	// Create function to decrement metrics
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION decrement_metric(
	p_resource TEXT,
	p_resource_id UUID,
	p_name TEXT
) RETURNS VOID AS $$
BEGIN
	INSERT INTO metrics (resource, resource_id, name, value)
	VALUES (p_resource, p_resource_id, p_name, 0)
	ON CONFLICT (resource, resource_id, name)
	DO UPDATE SET value = GREATEST(metrics.value - 1, 0);
END;
$$ LANGUAGE plpgsql;
`,
	}); err != nil {
		return err
	}

	// Trigger for likes INSERT
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION trigger_like_insert() RETURNS TRIGGER AS $$
BEGIN
	PERFORM increment_metric(NEW.likeable, NEW.likeable_id::UUID, 'likes');
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER likes_insert_trigger
AFTER INSERT ON likes
FOR EACH ROW
EXECUTE FUNCTION trigger_like_insert();
`,
	}); err != nil {
		return err
	}

	// Trigger for likes DELETE
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION trigger_like_delete() RETURNS TRIGGER AS $$
BEGIN
	PERFORM decrement_metric(OLD.likeable, OLD.likeable_id::UUID, 'likes');
	RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER likes_delete_trigger
AFTER DELETE ON likes
FOR EACH ROW
EXECUTE FUNCTION trigger_like_delete();
`,
	}); err != nil {
		return err
	}

	// Trigger for comments INSERT
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION trigger_comment_insert() RETURNS TRIGGER AS $$
BEGIN
	-- If it's a top-level comment (no parent_id), increment the post's comments metric
	IF NEW.parent_id IS NULL THEN
		PERFORM increment_metric(NEW.commentable, NEW.commentable_id::UUID, 'comments');
	ELSE
		-- If it's a reply (has parent_id), increment the parent comment's replies metric
		PERFORM increment_metric('comment', NEW.parent_id::UUID, 'replies');
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER comments_insert_trigger
AFTER INSERT ON comment
FOR EACH ROW
EXECUTE FUNCTION trigger_comment_insert();
`,
	}); err != nil {
		return err
	}

	// Trigger for comments DELETE
	if err := migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `
CREATE OR REPLACE FUNCTION trigger_comment_delete() RETURNS TRIGGER AS $$
BEGIN
	-- If it's a top-level comment (no parent_id), decrement the post's comments metric
	IF OLD.parent_id IS NULL THEN
		PERFORM decrement_metric(OLD.commentable, OLD.commentable_id::UUID, 'comments');
	ELSE
		-- If it's a reply (has parent_id), decrement the parent comment's replies metric
		PERFORM decrement_metric('comment', OLD.parent_id::UUID, 'replies');
	END IF;
	RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER comments_delete_trigger
AFTER DELETE ON comment
FOR EACH ROW
EXECUTE FUNCTION trigger_comment_delete();
`,
	}); err != nil {
		return err
	}

	return nil
}

func metricTriggersDown(ctx context.Context, db database.Database) error {
	// Only drop triggers for PostgreSQL
	if db.DriverName() != "postgres" {
		return nil
	}

	// Drop triggers
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP TRIGGER IF EXISTS likes_insert_trigger ON likes`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP TRIGGER IF EXISTS likes_delete_trigger ON likes`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP TRIGGER IF EXISTS comments_insert_trigger ON comment`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP TRIGGER IF EXISTS comments_delete_trigger ON comment`,
	})

	// Drop functions
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS trigger_like_insert()`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS trigger_like_delete()`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS trigger_comment_insert()`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS trigger_comment_delete()`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS increment_metric(TEXT, UUID, TEXT)`,
	})
	_ = migrations.SQL(ctx, db, migrations.DialectSQL{
		Postgres: `DROP FUNCTION IF EXISTS decrement_metric(TEXT, UUID, TEXT)`,
	})

	return nil
}
