package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chat/internal/model"
	"chat/internal/repository"
)

type repo struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) repository.ChatRepository {
	return &repo{db: db}
}


func insertOutboxEvents(ctx context.Context, tx pgx.Tx, events []repository.OutboxEvent) error {
	for _, ev := range events {
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, partition_key, payload) VALUES ($1, $2, $3)`,
			ev.Topic, ev.Key, ev.Payload,
		); err != nil {
			return fmt.Errorf("outbox insert: %w", err)
		}
	}
	return nil
}


func (r *repo) CreateChat(ctx context.Context, ownerID int64, name, description string, memberIDs []int64, buildEvents func(chatID int64) ([]repository.OutboxEvent, error)) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var chatID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO chats (owner_id, name, description) VALUES ($1, $2, $3) RETURNING id`,
		ownerID, name, description,
	).Scan(&chatID)
	if err != nil {
		return 0, fmt.Errorf("create chat: %w", err)
	}

	seen := map[int64]bool{ownerID: true}
	if _, err := tx.Exec(ctx,
		`INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, $3)`,
		chatID, ownerID, 2,
	); err != nil {
		return 0, err
	}
	for _, uid := range memberIDs {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		if _, err := tx.Exec(ctx,
			`INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, $3)`,
			chatID, uid, 0,
		); err != nil {
			return 0, err
		}
	}
	if buildEvents != nil {
		events, err := buildEvents(chatID)
		if err != nil {
			return 0, err
		}
		if err := insertOutboxEvents(ctx, tx, events); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return chatID, nil
}

func (r *repo) SoftDeleteChat(ctx context.Context, chatID, actorID int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE chats SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		chatID, actorID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chat not found or not owner")
	}
	return nil
}

func (r *repo) GetChat(ctx context.Context, chatID, userID int64) (*model.Chat, []int64, error) {
	ok, err := r.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("not a member")
	}
	var c model.Chat
	err = r.db.QueryRow(ctx, `
		SELECT c.id, c.owner_id, c.name, c.description, c.last_message_at, c.last_message_id, c.created_at,
		       COALESCE((SELECT COUNT(*) FROM messages m
		         WHERE m.chat_id = c.id AND m.deleted_at IS NULL
		           AND m.id > COALESCE(cm.last_read_message_id, 0)
		           AND m.sender_id <> $2), 0)
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $2
		WHERE c.id = $1 AND c.deleted_at IS NULL`,
		chatID, userID,
	).Scan(&c.ID, &c.OwnerID, &c.Name, &c.Description, &c.LastMessageAt, &c.LastMessageID, &c.CreatedAt, &c.UnreadCount)
	if err == pgx.ErrNoRows {
		return nil, nil, fmt.Errorf("chat not found")
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT user_id FROM chat_members WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	return &c, ids, rows.Err()
}

func (r *repo) ListChats(ctx context.Context, userID int64, cursor string, limit int32) ([]model.Chat, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var cursorTime *time.Time
	var cursorID int64
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) == 2 {
			if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				cursorTime = &t
			}
			cursorID, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	q := `
		SELECT c.id, c.owner_id, c.name, c.description, c.last_message_at, c.last_message_id, c.created_at,
		       COALESCE((SELECT COUNT(*) FROM messages m
		         WHERE m.chat_id = c.id AND m.deleted_at IS NULL
		           AND m.id > COALESCE(cm.last_read_message_id, 0)
		           AND m.sender_id <> $1), 0)
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
		WHERE c.deleted_at IS NULL`
	args := []any{userID}
	argN := 2
	if cursorTime != nil {
		q += fmt.Sprintf(` AND (COALESCE(c.last_message_at, c.created_at), c.id) < ($%d, $%d)`, argN, argN+1)
		args = append(args, *cursorTime, cursorID)
		argN += 2
	}
	q += fmt.Sprintf(` ORDER BY COALESCE(c.last_message_at, c.created_at) DESC, c.id DESC LIMIT $%d`, argN)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var chats []model.Chat
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, &c.Description, &c.LastMessageAt, &c.LastMessageID, &c.CreatedAt, &c.UnreadCount); err != nil {
			return nil, "", err
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	var next string
	if int32(len(chats)) > limit {
		chats = chats[:limit]
		last := chats[len(chats)-1]
		ts := last.CreatedAt
		if last.LastMessageAt != nil {
			ts = *last.LastMessageAt
		}
		next = ts.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(last.ID, 10)
	}
	return chats, next, nil
}

func (r *repo) ListChatIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
		WHERE c.deleted_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *repo) AddMember(ctx context.Context, chatID, userID int64, role int32, events []repository.OutboxEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		chatID, userID, role); err != nil {
		return err
	}
	if err := insertOutboxEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *repo) RemoveMember(ctx context.Context, chatID, userID int64, events []repository.OutboxEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`, chatID, userID); err != nil {
		return err
	}
	if err := insertOutboxEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *repo) UpdateMemberRole(ctx context.Context, chatID, userID int64, role int32, events []repository.OutboxEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE chat_members SET role = $3 WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("member not found")
	}
	if err := insertOutboxEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *repo) IsMember(ctx context.Context, chatID, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`,
		chatID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *repo) SendMessage(ctx context.Context, chatID, senderID int64, text, idemKey string, attachments []string, buildEvent func(m *model.Message) (*repository.OutboxEvent, error)) (*model.Message, bool, error) {
	if attachments == nil {
		attachments = []string{}
	}
	if idemKey != "" {
		var existing model.Message
		err := r.db.QueryRow(ctx, `
			SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids
			FROM messages WHERE chat_id = $1 AND sender_id = $2 AND idempotency_key = $3 AND deleted_at IS NULL`,
			chatID, senderID, idemKey,
		).Scan(&existing.ID, &existing.ChatID, &existing.SenderID, &existing.Text, &existing.IsPinned,
			&existing.SendAt, &existing.UpdatedAt, &existing.AttachmentIDs)
		if err == nil {
			return &existing, true, nil
		}
		if err != pgx.ErrNoRows {
			return nil, false, err
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	var m model.Message
	var key any
	if idemKey == "" {
		key = nil
	} else {
		key = idemKey
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO messages (chat_id, sender_id, text, idempotency_key, attachment_ids)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids`,
		chatID, senderID, text, key, attachments,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs)
	if err != nil {
		if idemKey != "" && strings.Contains(err.Error(), "messages_idempotency_idx") {
			_ = tx.Rollback(ctx)
			var existing model.Message
			qerr := r.db.QueryRow(ctx, `
				SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids
				FROM messages WHERE chat_id = $1 AND sender_id = $2 AND idempotency_key = $3 AND deleted_at IS NULL`,
				chatID, senderID, idemKey,
			).Scan(&existing.ID, &existing.ChatID, &existing.SenderID, &existing.Text, &existing.IsPinned,
				&existing.SendAt, &existing.UpdatedAt, &existing.AttachmentIDs)
			if qerr == nil {
				return &existing, true, nil
			}
		}
		return nil, false, fmt.Errorf("insert message: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE chats SET last_message_at = $2, last_message_id = $3, updated_at = NOW() WHERE id = $1`,
		chatID, m.SendAt, m.ID)
	if err != nil {
		return nil, false, err
	}
	if buildEvent != nil {
		ev, err := buildEvent(&m)
		if err != nil {
			return nil, false, err
		}
		if ev != nil {
			if err := insertOutboxEvents(ctx, tx, []repository.OutboxEvent{*ev}); err != nil {
				return nil, false, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return &m, false, nil
}

func (r *repo) EditMessage(ctx context.Context, messageID, actorID int64, text string, buildEvents func(m *model.Message) ([]repository.OutboxEvent, error)) (*model.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var m model.Message
	err = tx.QueryRow(ctx, `
		UPDATE messages SET text = $3, updated_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids`,
		messageID, actorID, text,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(&m)
		if err != nil {
			return nil, err
		}
		if err := insertOutboxEvents(ctx, tx, events); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &m, nil
}



func (r *repo) DeleteMessage(ctx context.Context, messageID, actorID int64, buildEvents func(m *model.Message) ([]repository.OutboxEvent, error)) (*model.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var m model.Message
	err = tx.QueryRow(ctx, `
		UPDATE messages SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids`,
		messageID, actorID,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(&m)
		if err != nil {
			return nil, err
		}
		if err := insertOutboxEvents(ctx, tx, events); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &m, nil
}



func (r *repo) PinMessage(ctx context.Context, messageID, actorID int64, pinned bool, buildEvents func(m *model.Message) ([]repository.OutboxEvent, error)) (*model.Message, error) {
	var chatID int64
	err := r.db.QueryRow(ctx, `SELECT chat_id FROM messages WHERE id = $1 AND deleted_at IS NULL`, messageID).Scan(&chatID)
	if err != nil {
		return nil, fmt.Errorf("message not found")
	}
	ok, err := r.IsMember(ctx, chatID, actorID)
	if err != nil || !ok {
		return nil, fmt.Errorf("not a member")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var m model.Message
	err = tx.QueryRow(ctx, `
		UPDATE messages SET is_pinned = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids`,
		messageID, pinned,
	).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs)
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(&m)
		if err != nil {
			return nil, err
		}
		if err := insertOutboxEvents(ctx, tx, events); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &m, nil
}



func (r *repo) ListMessages(ctx context.Context, chatID, userID, beforeID int64, limit int32) ([]model.Message, bool, error) {
	ok, err := r.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("not a member")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids
		FROM messages WHERE chat_id = $1 AND deleted_at IS NULL`
	args := []any{chatID}
	if beforeID > 0 {
		q += ` AND id < $2`
		args = append(args, beforeID)
		q += fmt.Sprintf(` ORDER BY id DESC LIMIT $%d`, len(args)+1)
	} else {
		q += ` ORDER BY id DESC LIMIT $2`
	}
	args = append(args, limit+1)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs); err != nil {
			return nil, false, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := int32(len(msgs)) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	return msgs, hasMore, nil
}

func (r *repo) MarkRead(ctx context.Context, chatID, userID, messageID int64, buildEvents func() ([]repository.OutboxEvent, error)) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE chat_members SET last_read_message_id = GREATEST(last_read_message_id, $3)
		WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID, messageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not a member")
	}
	if buildEvents != nil {
		events, err := buildEvents()
		if err != nil {
			return err
		}
		if err := insertOutboxEvents(ctx, tx, events); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}



func (r *repo) GetUnreadCounts(ctx context.Context, userID int64, chatIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64)
	if len(chatIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT cm.chat_id,
		       COALESCE((SELECT COUNT(*) FROM messages m
		         WHERE m.chat_id = cm.chat_id AND m.deleted_at IS NULL
		           AND m.id > COALESCE(cm.last_read_message_id, 0)
		           AND m.sender_id <> $1), 0)
		FROM chat_members cm
		WHERE cm.user_id = $1 AND cm.chat_id = ANY($2)`,
		userID, chatIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, cnt int64
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		out[id] = cnt
	}
	return out, rows.Err()
}
