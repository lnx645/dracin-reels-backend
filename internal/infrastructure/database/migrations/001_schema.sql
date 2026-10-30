-- Dracin Reels — schema (idempotent, safe to re-run).
-- PostgreSQL 13+. All timestamps TIMESTAMPTZ, default now().

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    avatar_url TEXT,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS genres (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS series (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cover_url TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    total_episode INT NOT NULL DEFAULT 0,
    free_episode_count INT NOT NULL DEFAULT 0,
    rating NUMERIC(3,1) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS series_genres (
    series_id BIGINT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    genre_id BIGINT NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (series_id, genre_id)
);

CREATE TABLE IF NOT EXISTS episodes (
    id BIGSERIAL PRIMARY KEY,
    series_id BIGINT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    number INT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    video_url TEXT NOT NULL DEFAULT '',
    subtitle_url TEXT,
    duration_seconds INT NOT NULL DEFAULT 0,
    coin_price INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    thumbnail_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (series_id, number)
);

CREATE TABLE IF NOT EXISTS coin_wallets (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance INT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS coin_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('checkin', 'task', 'purchase', 'unlock', 'reward')),
    amount INT NOT NULL,
    balance_after INT NOT NULL,
    reference_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_coin_transactions_user ON coin_transactions(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS coin_packages (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    coin_amount INT NOT NULL,
    price_idr INT NOT NULL,
    bonus_coins INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vip_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan TEXT NOT NULL CHECK (plan IN ('weekly', 'monthly', 'yearly')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired')),
    auto_renew BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vip_user ON vip_subscriptions(user_id, ends_at DESC);

CREATE TABLE IF NOT EXISTS watch_progress (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id BIGINT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    position_seconds INT NOT NULL DEFAULT 0,
    finished BOOLEAN NOT NULL DEFAULT false,
    last_watched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, episode_id)
);

CREATE TABLE IF NOT EXISTS bookmarks (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    series_id BIGINT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, series_id)
);

CREATE TABLE IF NOT EXISTS likes (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id BIGINT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, episode_id)
);

CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id BIGINT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_comments_episode ON comments(episode_id, created_at ASC);

CREATE TABLE IF NOT EXISTS reward_tasks (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('checkin', 'watch_ad', 'finish_series')),
    coin_reward INT NOT NULL DEFAULT 0,
    daily_limit INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_reward_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id BIGINT NOT NULL REFERENCES reward_tasks(id) ON DELETE CASCADE,
    claim_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, task_id, claim_date)
);

-- Supporting table (see DOMAIN_MODEL.md): records coin-unlocked episodes.
CREATE TABLE IF NOT EXISTS episode_unlocks (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id BIGINT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, episode_id)
);
