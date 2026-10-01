-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

ALTER TABLE users ADD COLUMN IF NOT EXISTS username CITEXT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_file_id TEXT NULL;

UPDATE users
SET username = ('user_' || id::text)::citext
WHERE username IS NULL;

ALTER TABLE users ALTER COLUMN username SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_username_unique_idx ON users (username);

-- +goose Down
DROP INDEX IF EXISTS users_username_unique_idx;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_file_id;
ALTER TABLE users DROP COLUMN IF EXISTS username;
