-- 005: per-user series ratings (1-5 stars) + likes on comments.
-- Idempotent: safe to re-run (IF NOT EXISTS / ON CONFLICT-free DDL).

CREATE TABLE IF NOT EXISTS series_ratings (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id  BIGINT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    stars      INT NOT NULL CHECK (stars BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, series_id)
);
CREATE INDEX IF NOT EXISTS idx_series_ratings_series ON series_ratings(series_id);

CREATE TABLE IF NOT EXISTS comment_likes (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment_id BIGINT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comment_id)
);
CREATE INDEX IF NOT EXISTS idx_comment_likes_comment ON comment_likes(comment_id);
