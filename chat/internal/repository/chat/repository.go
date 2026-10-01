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

func orderedPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func (r *repo) CreateChat(ctx context.Context, ownerID int64, name, description string, chatType int16, memberIDs []int64, buildEvents func(chatID int64) ([]repository.OutboxEvent, error)) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var chatID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO chats (owner_id, name, description, chat_type) VALUES ($1, $2, $3, $4) RETURNING id`,
		ownerID, name, description, chatType,
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
		if uid <= 0 || seen[uid] {
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

func (r *repo) GetOrCreateDirect(ctx context.Context, userA, userB int64, buildEvents func(chatID int64) ([]repository.OutboxEvent, error)) (int64, error) {
	a, b := orderedPair(userA, userB)
	var existing int64
	err := r.db.QueryRow(ctx, `SELECT chat_id FROM direct_chat_pairs WHERE user_a = $1 AND user_b = $2`, a, b).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// race: re-check inside tx
	err = tx.QueryRow(ctx, `SELECT chat_id FROM direct_chat_pairs WHERE user_a = $1 AND user_b = $2`, a, b).Scan(&existing)
	if err == nil {
		_ = tx.Rollback(ctx)
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}

	var chatID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO chats (owner_id, name, description, chat_type) VALUES ($1, '', '', $2) RETURNING id`,
		userA, model.ChatTypeDirect,
	).Scan(&chatID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, 2)`, chatID, userA); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, 0)`, chatID, userB); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO direct_chat_pairs (user_a, user_b, chat_id) VALUES ($1, $2, $3)`, a, b, chatID); err != nil {
		return 0, err
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

func (r *repo) UpdateChat(ctx context.Context, chatID, actorID int64, name, description, avatarFileID *string) error {
	ok, err := r.IsMember(ctx, chatID, actorID)
	if err != nil || !ok {
		return fmt.Errorf("not a member")
	}
	var typ int16
	if err := r.db.QueryRow(ctx, `SELECT chat_type FROM chats WHERE id = $1 AND deleted_at IS NULL`, chatID).Scan(&typ); err != nil {
		return fmt.Errorf("chat not found")
	}
	if typ == model.ChatTypeDirect {
		return fmt.Errorf("cannot update direct chat")
	}
	sets := []string{"updated_at = NOW()"}
	args := []any{chatID}
	n := 2
	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", n))
		args = append(args, *name)
		n++
	}
	if description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", n))
		args = append(args, *description)
		n++
	}
	if avatarFileID != nil {
		sets = append(sets, fmt.Sprintf("avatar_file_id = $%d", n))
		args = append(args, *avatarFileID)
		n++
	}
	if len(sets) == 1 {
		return nil
	}
	q := `UPDATE chats SET ` + strings.Join(sets, ", ") + ` WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chat not found")
	}
	return nil
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

func enrichChat(ctx context.Context, db *pgxpool.Pool, c *model.Chat, viewerID int64) error {
	rows, err := db.Query(ctx, `SELECT user_id FROM chat_members WHERE chat_id = $1`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	c.ParticipantIDs = ids
	if c.ChatType == model.ChatTypeDirect {
		for _, id := range ids {
			if id != viewerID {
				c.PeerUserID = id
				break
			}
		}
	}
	if c.LastMessageID != nil && *c.LastMessageID > 0 {
		var preview string
		_ = db.QueryRow(ctx, `SELECT left(text, 120) FROM messages WHERE id = $1`, *c.LastMessageID).Scan(&preview)
		c.LastMessagePreview = preview
	}
	return rows.Err()
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
		SELECT c.id, c.owner_id, c.name, c.description, c.chat_type, COALESCE(c.avatar_file_id, ''),
		       c.last_message_at, c.last_message_id, c.created_at,
		       COALESCE((SELECT COUNT(*) FROM messages m
		         WHERE m.chat_id = c.id AND m.deleted_at IS NULL
		           AND m.id > COALESCE(cm.last_read_message_id, 0)
		           AND m.sender_id <> $2), 0)
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $2
		WHERE c.id = $1 AND c.deleted_at IS NULL`,
		chatID, userID,
	).Scan(&c.ID, &c.OwnerID, &c.Name, &c.Description, &c.ChatType, &c.AvatarFileID, &c.LastMessageAt, &c.LastMessageID, &c.CreatedAt, &c.UnreadCount)
	if err == pgx.ErrNoRows {
		return nil, nil, fmt.Errorf("chat not found")
	}
	if err != nil {
		return nil, nil, err
	}
	if err := enrichChat(ctx, r.db, &c, userID); err != nil {
		return nil, nil, err
	}
	return &c, c.ParticipantIDs, nil
}

func (r *repo) GetChatType(ctx context.Context, chatID int64) (int16, error) {
	var t int16
	err := r.db.QueryRow(ctx, `SELECT chat_type FROM chats WHERE id = $1 AND deleted_at IS NULL`, chatID).Scan(&t)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("chat not found")
	}
	return t, err
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
		SELECT c.id, c.owner_id, c.name, c.description, c.chat_type, COALESCE(c.avatar_file_id, ''),
		       c.last_message_at, c.last_message_id, c.created_at,
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
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, &c.Description, &c.ChatType, &c.AvatarFileID, &c.LastMessageAt, &c.LastMessageID, &c.CreatedAt, &c.UnreadCount); err != nil {
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
	for i := range chats {
		if err := enrichChat(ctx, r.db, &chats[i], userID); err != nil {
			return nil, "", err
		}
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

func (r *repo) GetMemberRole(ctx context.Context, chatID, userID int64) (int32, error) {
	var role int32
	err := r.db.QueryRow(ctx,
		`SELECT role FROM chat_members WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID,
	).Scan(&role)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("not a member")
	}
	return role, err
}

func scanMsg(row pgx.Row) (*model.Message, error) {
	var m model.Message
	var reply *int64
	err := row.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs, &reply)
	if err != nil {
		return nil, err
	}
	if reply != nil {
		m.ReplyToMessageID = *reply
	}
	return &m, nil
}

func (r *repo) SendMessage(ctx context.Context, chatID, senderID int64, text, idemKey string, attachments []string, replyTo int64, buildEvent func(m *model.Message) (*repository.OutboxEvent, error)) (*model.Message, bool, error) {
	if attachments == nil {
		attachments = []string{}
	}
	if idemKey != "" {
		existing, err := scanMsg(r.db.QueryRow(ctx, `
			SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id
			FROM messages WHERE chat_id = $1 AND sender_id = $2 AND idempotency_key = $3 AND deleted_at IS NULL`,
			chatID, senderID, idemKey))
		if err == nil {
			return existing, true, nil
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

	var key any
	if idemKey == "" {
		key = nil
	} else {
		key = idemKey
	}
	var reply any
	if replyTo > 0 {
		reply = replyTo
	} else {
		reply = nil
	}
	m, err := scanMsg(tx.QueryRow(ctx, `
		INSERT INTO messages (chat_id, sender_id, text, idempotency_key, attachment_ids, reply_to_message_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id`,
		chatID, senderID, text, key, attachments, reply))
	if err != nil {
		if idemKey != "" && strings.Contains(err.Error(), "messages_idempotency_idx") {
			_ = tx.Rollback(ctx)
			existing, qerr := scanMsg(r.db.QueryRow(ctx, `
				SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id
				FROM messages WHERE chat_id = $1 AND sender_id = $2 AND idempotency_key = $3 AND deleted_at IS NULL`,
				chatID, senderID, idemKey))
			if qerr == nil {
				return existing, true, nil
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
		ev, err := buildEvent(m)
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
	return m, false, nil
}

func (r *repo) EditMessage(ctx context.Context, messageID, actorID int64, text string, buildEvents func(m *model.Message) ([]repository.OutboxEvent, error)) (*model.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	m, err := scanMsg(tx.QueryRow(ctx, `
		UPDATE messages SET text = $3, updated_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id`,
		messageID, actorID, text))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(m)
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
	return m, nil
}

func (r *repo) DeleteMessage(ctx context.Context, messageID, actorID int64, buildEvents func(m *model.Message) ([]repository.OutboxEvent, error)) (*model.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	m, err := scanMsg(tx.QueryRow(ctx, `
		UPDATE messages SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id`,
		messageID, actorID))
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(m)
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
	return m, nil
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
	m, err := scanMsg(tx.QueryRow(ctx, `
		UPDATE messages SET is_pinned = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id`,
		messageID, pinned))
	if err != nil {
		return nil, err
	}
	if buildEvents != nil {
		events, err := buildEvents(m)
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
	return m, nil
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
		SELECT id, chat_id, sender_id, text, is_pinned, send_at, updated_at, attachment_ids, reply_to_message_id
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
		var reply *int64
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Text, &m.IsPinned, &m.SendAt, &m.UpdatedAt, &m.AttachmentIDs, &reply); err != nil {
			return nil, false, err
		}
		if reply != nil {
			m.ReplyToMessageID = *reply
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
