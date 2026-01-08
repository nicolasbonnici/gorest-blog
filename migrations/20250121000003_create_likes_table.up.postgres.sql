-- Create polymorphic likes table
CREATE TABLE likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    liker_id UUID REFERENCES users(id) ON DELETE CASCADE,
    liked_id UUID REFERENCES users(id),
    likeable TEXT CHECK (likeable IN ('post', 'comment')) NOT NULL,
    likeable_id UUID NOT NULL,
    liked_at TIMESTAMP(0) WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP(0) WITH TIME ZONE,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_likes_liker ON likes (liker_id);
CREATE INDEX idx_likes_composite ON likes (likeable, likeable_id, liked_at);
CREATE INDEX idx_likes_liked_at ON likes (liked_at);
CREATE UNIQUE INDEX uniq_likes_liker_likeable ON likes (liker_id, likeable, likeable_id);
