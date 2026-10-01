package service

import (
	"context"

	"chat/internal/model"
)

type ChatService interface {
	CreateChat(ctx context.Context, actorID int64, name, description string, memberIDs []int64) (int64, error)
	DeleteChat(ctx context.Context, actorID, chatID int64) error
	GetChat(ctx context.Context, actorID, chatID int64) (*model.Chat, []int64, error)
	ListChats(ctx context.Context, actorID int64, cursor string, limit int32) ([]model.Chat, string, error)
	ListChatIDs(ctx context.Context, actorID int64) ([]int64, error)
	AddUser(ctx context.Context, actorID, chatID, userID int64, role int32) error
	RemoveUser(ctx context.Context, actorID, chatID, userID int64) error
	UpdateUserRole(ctx context.Context, actorID, chatID, userID int64, role int32) error
	SendMessage(ctx context.Context, actorID, chatID int64, text, idemKey string, attachments []string) (int64, error)
	EditMessage(ctx context.Context, actorID, messageID int64, text string) error
	DeleteMessage(ctx context.Context, actorID, messageID int64) error
	PinMessage(ctx context.Context, actorID, messageID int64, pinned bool) error
	ListMessages(ctx context.Context, actorID, chatID, beforeID int64, limit int32) ([]model.Message, bool, error)
	MarkRead(ctx context.Context, actorID, chatID, messageID int64) error
	GetUnreadCounts(ctx context.Context, actorID int64, chatIDs []int64) (map[int64]int64, error)
}
