-- +goose Up
ALTER TABLE outbox
  ADD COLUMN IF NOT EXISTS kafka_published_at TIMESTAMPTZ NULL,
  ADD COLUMN IF NOT EXISTS realtime_published_at TIMESTAMPTZ NULL;

-- Rows already fully published keep published_at; backfill stages for consistency.
UPDATE outbox
SET kafka_published_at = COALESCE(kafka_published_at, published_at),
    realtime_published_at = COALESCE(realtime_published_at, published_at)
WHERE published_at IS NOT NULL;

DROP INDEX IF EXISTS outbox_pending_idx;
CREATE INDEX IF NOT EXISTS outbox_pending_idx
  ON outbox (id)
  WHERE published_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS outbox_pending_idx;
CREATE INDEX IF NOT EXISTS outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
ALTER TABLE outbox
  DROP COLUMN IF EXISTS realtime_published_at,
  DROP COLUMN IF EXISTS kafka_published_at;
