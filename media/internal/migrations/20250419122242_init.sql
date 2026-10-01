-- +goose Up
CREATE TABLE IF NOT EXISTS files (
  id UUID PRIMARY KEY,
  owner_id BIGINT NOT NULL,
  object_key TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime TEXT NOT NULL DEFAULT '',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS files_owner_idx ON files (owner_id);

-- +goose Down
DROP TABLE IF EXISTS files;
