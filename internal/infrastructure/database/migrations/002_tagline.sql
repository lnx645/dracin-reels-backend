-- Tagline for series (genre + tagline display feature).
-- Idempotent: safe to re-run on every boot.
-- NOTE: empty default uses dollar-quoting, not single quotes.
ALTER TABLE series ADD COLUMN IF NOT EXISTS tagline TEXT NOT NULL DEFAULT $$$$;
