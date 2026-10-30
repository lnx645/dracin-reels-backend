-- Dracin Reels — admin panel (idempotent, safe to re-run).
-- Users gain an active flag: deactivation is the soft off-switch
-- (never a hard delete, never a password/hash change).

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_users_created ON users(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coin_transactions_created ON coin_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vip_effective ON vip_subscriptions(status, ends_at DESC);
