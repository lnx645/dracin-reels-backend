-- HLS adaptive playback: per-episode HLS URL + transcode status.
-- Idempotent: safe to re-run on every boot.
-- transcode_status: 'none' (no HLS job, e.g. external link), 'processing',
-- 'ready' (hls_url playable), 'failed' (reason in transcode_error, the
-- original file stays playable as fallback — a failure NEVER fabricates
-- an HLS URL).
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS hls_url TEXT;
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS transcode_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS transcode_error TEXT;
