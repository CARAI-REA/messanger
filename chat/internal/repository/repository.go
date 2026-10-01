package repository

import (
	"context"
	"time"

	"chat/internal/model"
)

type OutboxEvent struct {
	Topic   string
	Key     string
	Payload []byte
}

type ChatRepository interface {
	CreateChat(ctx context.Context, ownerID int64, name, description string, memberIDs []int64, buildEvents func(chatID int64) ([]OutboxEvent, error)) (int64, error)
	SoftDeleteChat(ctx context.Context, chatID, actorID int64) error
	GetChat(ctx context.Context, chatID, userID int64) (*model.Chat, []int64, error)
	ListChats(ctx context.Context, userID int64, cursor string, limit int32) ([]model.Chat, string, error)
	ListChatIDs(ctx context.Context, userID int64) ([]int64, error)
	AddMember(ctx context.Context, chatID, userID int64, role int32, events []OutboxEvent) error
	RemoveMember(ctx context.Context, chatID, userID int64, events []OutboxEvent) error
	UpdateMemberRole(ctx context.Context, chatID, userID int64, role int32, events []OutboxEvent) error
	IsMember(ctx context.Context, chatID, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID, senderID int64, text, idemKey string, attachments []string, buildEvent func(m *model.Message) (*OutboxEvent, error)) (*model.Message, bool, error)
	EditMessage(ctx context.Context, messageID, actorID int64, text string, buildEvents func(m *model.Message) ([]OutboxEvent, error)) (*model.Message, error)
	DeleteMessage(ctx context.Context, messageID, actorID int64, buildEvents func(m *model.Message) ([]OutboxEvent, error)) (*model.Message, error)
	PinMessage(ctx context.Context, messageID, actorID int64, pinned bool, buildEvents func(m *model.Message) ([]OutboxEvent, error)) (*model.Message, error)
	ListMessages(ctx context.Context, chatID, userID, beforeID int64, limit int32) ([]model.Message, bool, error)
	MarkRead(ctx context.Context, chatID, userID, messageID int64, buildEvents func() ([]OutboxEvent, error)) error
	GetUnreadCounts(ctx context.Context, userID int64, chatIDs []int64) (map[int64]int64, error)
}

type OutboxRepository interface {
	Insert(ctx context.Context, topic, key string, payload []byte) error
	ClaimPending(ctx context.Context, limit int) ([]OutboxRow, error)
	MarkKafkaPublished(ctx context.Context, id int64) error
	MarkRealtimePublished(ctx context.Context, id int64) error
	MarkFullyPublished(ctx context.Context, id int64) error
	CountPending(ctx context.Context) (int64, error)
}

type OutboxRow struct {
	ID                  int64
	Topic               string
	Key                 string
	Payload             []byte
	KafkaPublishedAt    *time.Time
	RealtimePublishedAt *time.Time
}
