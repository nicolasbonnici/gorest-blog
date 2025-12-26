-- Rollback comments table
DROP INDEX IF EXISTS idx_comment_parent;
DROP INDEX IF EXISTS idx_comment_post;
DROP INDEX IF EXISTS idx_comment_user;
DROP TABLE IF EXISTS comment CASCADE;
