-- +goose Up
ALTER TABLE chats ADD COLUMN IF NOT EXISTS chat_type SMALLINT NOT NULL DEFAULT 2;

-- Existing multi-member chats stay GROUP (2). Direct chats created going forward use 1.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_message_id BIGINT NULL;

CREATE TABLE IF NOT EXISTS direct_chat_pairs (
  user_a BIGINT NOT NULL,
  user_b BIGINT NOT NULL,
  chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  PRIMARY KEY (user_a, user_b),
  CONSTRAINT direct_chat_pairs_ordered CHECK (user_a < user_b)
);

CREATE UNIQUE INDEX IF NOT EXISTS direct_chat_pairs_chat_idx ON direct_chat_pairs (chat_id);

-- +goose Down
DROP INDEX IF EXISTS direct_chat_pairs_chat_idx;
DROP TABLE IF EXISTS direct_chat_pairs;
ALTER TABLE messages DROP COLUMN IF EXISTS reply_to_message_id;
ALTER TABLE chats DROP COLUMN IF EXISTS chat_type;
