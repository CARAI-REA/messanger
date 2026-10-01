package model

import "time"

type UserInfo struct {
	Name  string
	Email string
}

type User struct {
	ID        int64
	Info      UserInfo
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserCreatedEvent struct {
	UserID    int64
	CreatedAt time.Time
}

type UserDeletedEvent struct {
	UserID    int64
	DeletedAt time.Time
}
