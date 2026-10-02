package model

import "time"

const (
	ChatTypeDirect = 1
	ChatTypeGroup  = 2
)

type Chat struct {
	ID                 int64
	OwnerID            int64
	Name               string
	Description        string
	ChatType           int16
	AvatarFileID       string
	IsPinned           bool
	LastMessageAt      *time.Time
	LastMessageID      *int64
	LastMessagePreview string
	UnreadCount        int64
	ParticipantIDs     []int64
	PeerUserID         int64
	CreatedAt          time.Time
}

type Message struct {
	ID               int64
	ChatID           int64
	SenderID         int64
	Text             string
	IsPinned         bool
	SendAt           time.Time
	UpdatedAt        time.Time
	AttachmentIDs    []string
	ReplyToMessageID int64
}
