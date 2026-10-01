package repository

import (
	"context"
	"time"

	"user/internal/model"
)

type OutboxEvent struct {
	Topic   string
	Key     string
	Payload []byte
}

type UserUpdate struct {
	Name         *string
	Email        *string
	Username     *string
	AvatarFileID *string
}

type UserRepository interface {
	Create(ctx context.Context, info *model.UserInfo, passwordHash string, createdAt time.Time, buildEvents func(id int64) ([]OutboxEvent, error)) (int64, error)
	Get(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, string, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	Search(ctx context.Context, query string, limit int) ([]*model.User, error)
	Update(ctx context.Context, id int64, upd UserUpdate) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string, ip string) error
	Delete(ctx context.Context, id int64, buildEvents func() ([]OutboxEvent, error)) error
}

type OutboxRepository interface {
	Insert(ctx context.Context, topic, key string, payload []byte) error
	ClaimPending(ctx context.Context, limit int) ([]OutboxRow, error)
	MarkKafkaPublished(ctx context.Context, id int64) error
	MarkFullyPublished(ctx context.Context, id int64) error
	CountPending(ctx context.Context) (int64, error)
}

type OutboxRow struct {
	ID               int64
	Topic            string
	Key              string
	Payload          []byte
	KafkaPublishedAt *time.Time
}
