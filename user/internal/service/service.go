package service

import (
	"context"

	"user/internal/model"
	"user/internal/repository"
)

type UserService interface {
	Create(ctx context.Context, info model.UserInfo, password, passwordConfirm string) (int64, error)
	Get(ctx context.Context, id int64) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	Search(ctx context.Context, query string, limit int) ([]*model.User, error)
	Update(ctx context.Context, id int64, upd repository.UserUpdate) error
	UpdatePassword(ctx context.Context, id int64, password, passwordConfirm, ip string) error
	Delete(ctx context.Context, id int64) error
	ValidateCredentials(ctx context.Context, email, password string) (bool, int64, error)
}
