-- +goose Up
ALTER TABLE chat_members ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS chat_members_user_pinned_idx ON chat_members (user_id, is_pinned DESC);

-- +goose Down
DROP INDEX IF EXISTS chat_members_user_pinned_idx;
ALTER TABLE chat_members DROP COLUMN IF EXISTS is_pinned;
