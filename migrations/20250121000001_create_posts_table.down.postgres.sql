-- Rollback posts table and enum
DROP INDEX IF EXISTS idx_post_slug;
DROP INDEX IF EXISTS idx_post_fk_user;
DROP INDEX IF EXISTS idx_post_status;
DROP INDEX IF EXISTS idx_post_title;
DROP TABLE IF EXISTS post CASCADE;
DROP TYPE IF EXISTS post_status CASCADE;
