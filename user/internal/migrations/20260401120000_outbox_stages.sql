-- +goose Up
ALTER TABLE outbox
  ADD COLUMN IF NOT EXISTS kafka_published_at TIMESTAMPTZ NULL;

UPDATE outbox
SET kafka_published_at = COALESCE(kafka_published_at, published_at)
WHERE published_at IS NOT NULL;

DROP INDEX IF EXISTS outbox_pending_idx;
CREATE INDEX IF NOT EXISTS outbox_pending_idx
  ON outbox (id)
  WHERE published_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS outbox_pending_idx;
CREATE INDEX IF NOT EXISTS outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
ALTER TABLE outbox DROP COLUMN IF EXISTS kafka_published_at;
