-- Tagline for series (genre + tagline display feature).
-- Idempotent: safe to re-run on every boot.
ALTER TABLE series ADD COLUMN IF NOT EXISTS tagline TEXT NOT NULL DEFAULT ;
