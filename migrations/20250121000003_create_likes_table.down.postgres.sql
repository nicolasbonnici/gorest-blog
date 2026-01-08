-- Rollback likes table
DROP INDEX IF EXISTS uniq_likes_liker_likeable;
DROP INDEX IF EXISTS idx_likes_liked_at;
DROP INDEX IF EXISTS idx_likes_composite;
DROP INDEX IF EXISTS idx_likes_liker;
DROP TABLE IF EXISTS likes CASCADE;
