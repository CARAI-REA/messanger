-- +goose Up
CREATE TABLE IF NOT EXISTS devices (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  push_token TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, push_token)
);

CREATE INDEX IF NOT EXISTS devices_user_idx ON devices (user_id);

-- +goose Down
DROP TABLE IF EXISTS devices;
