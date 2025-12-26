-- Create post_status enum and posts table
CREATE TYPE post_status AS ENUM ('drafted', 'published');

CREATE TABLE post (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    slug TEXT NOT NULL,
    status post_status NOT NULL DEFAULT 'drafted',
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    published_at TIMESTAMP(0) WITH TIME ZONE,
    updated_at TIMESTAMP(0) WITH TIME ZONE,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_post_title ON post (title);
CREATE INDEX idx_post_status ON post (status);
CREATE INDEX idx_post_fk_user ON post (user_id);
CREATE INDEX idx_post_slug ON post (slug);
