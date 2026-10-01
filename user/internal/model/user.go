package model

import "time"

type UserInfo struct {
	Name     string
	Email    string
	Username string
}

type User struct {
	ID           int64
	Info         UserInfo
	AvatarFileID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserCreatedEvent struct {
	UserID    int64
	CreatedAt time.Time
}

type UserDeletedEvent struct {
	UserID    int64
	DeletedAt time.Time
}
