-- Create comments table with hierarchical support
CREATE TABLE comment (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    post_id UUID REFERENCES post(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES comment(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    updated_at TIMESTAMP(0) WITH TIME ZONE,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_comment_user ON comment (user_id);
CREATE INDEX idx_comment_post ON comment (post_id);
CREATE INDEX idx_comment_parent ON comment (parent_id);
