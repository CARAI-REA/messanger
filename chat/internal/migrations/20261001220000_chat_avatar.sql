-- +goose Up
ALTER TABLE chats ADD COLUMN IF NOT EXISTS avatar_file_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE chats DROP COLUMN IF EXISTS avatar_file_id;
