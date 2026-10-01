package model

import "time"

type Chat struct {
	ID            int64
	OwnerID       int64
	Name          string
	Description   string
	LastMessageAt *time.Time
	LastMessageID *int64
	UnreadCount   int64
	CreatedAt     time.Time
}

type Message struct {
	ID            int64
	ChatID        int64
	SenderID      int64
	Text          string
	IsPinned      bool
	SendAt        time.Time
	UpdatedAt     time.Time
	AttachmentIDs []string
}
