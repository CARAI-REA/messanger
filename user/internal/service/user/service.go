package user

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	eventsv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/events/v1"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"user/internal/config"
	"user/internal/model"
	"user/internal/repository"
	"user/internal/service"
)

var (
	usernameRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,31}$`)
	reserved   = map[string]bool{
		"admin": true, "support": true, "system": true, "me": true,
		"api": true, "root": true, "null": true, "undefined": true,
	}
)

type serv struct {
	repo repository.UserRepository
}

func NewService(repo repository.UserRepository, _ repository.OutboxRepository) service.UserService {
	return &serv{repo: repo}
}

func NormalizeUsername(u string) (string, error) {
	u = strings.TrimSpace(strings.TrimPrefix(u, "@"))
	if !usernameRE.MatchString(u) {
		return "", fmt.Errorf("username must be 3-32 chars, start with a letter, and use only letters, digits, underscore")
	}
	lower := strings.ToLower(u)
	if reserved[lower] {
		return "", fmt.Errorf("username is reserved")
	}
	return lower, nil
}

func (s *serv) hashPassword(password, confirm string) (string, error) {
	if password == "" || confirm == "" {
		return "", fmt.Errorf("password required")
	}
	if password != confirm {
		return "", fmt.Errorf("passwords do not match")
	}
	if len(password) < 8 {
		return "", fmt.Errorf("password too short")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *serv) Create(ctx context.Context, info model.UserInfo, password, passwordConfirm string) (int64, error) {
	if info.Name == "" || info.Email == "" {
		return 0, fmt.Errorf("name and email required")
	}
	uname, err := NormalizeUsername(info.Username)
	if err != nil {
		return 0, err
	}
	info.Username = uname
	hash, err := s.hashPassword(password, passwordConfirm)
	if err != nil {
		return 0, err
	}
	createdAt := time.Now().UTC()
	return s.repo.Create(ctx, &info, hash, createdAt, func(id int64) ([]repository.OutboxEvent, error) {
		ev := &eventsv1.UserCreated{UserId: id, CreatedAt: timestamppb.New(createdAt)}
		payload, err := proto.Marshal(ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{{
			Topic:   config.AppConfig().Kafka.UserCreatedTopic(),
			Key:     strconv.FormatInt(id, 10),
			Payload: payload,
		}}, nil
	})
}

func (s *serv) Get(ctx context.Context, id int64) (*model.User, error) {
	return s.repo.Get(ctx, id)
}

func (s *serv) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	uname, err := NormalizeUsername(username)
	if err != nil {
		// allow lookup of existing handles even if validation tightened; still normalize
		uname = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
		if uname == "" {
			return nil, fmt.Errorf("username required")
		}
	}
	return s.repo.GetByUsername(ctx, uname)
}

func (s *serv) Search(ctx context.Context, query string, limit int) ([]*model.User, error) {
	return s.repo.Search(ctx, query, limit)
}

func (s *serv) Update(ctx context.Context, id int64, upd repository.UserUpdate) error {
	if upd.Username != nil {
		uname, err := NormalizeUsername(*upd.Username)
		if err != nil {
			return err
		}
		upd.Username = &uname
	}
	return s.repo.Update(ctx, id, upd)
}

func (s *serv) UpdatePassword(ctx context.Context, id int64, password, passwordConfirm, ip string) error {
	hash, err := s.hashPassword(password, passwordConfirm)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, id, hash, ip)
}

func (s *serv) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id, func() ([]repository.OutboxEvent, error) {
		ev := &eventsv1.UserDeleted{UserId: id, DeletedAt: timestamppb.Now()}
		payload, err := proto.Marshal(ev)
		if err != nil {
			return nil, err
		}
		return []repository.OutboxEvent{{
			Topic:   config.AppConfig().Kafka.UserDeletedTopic(),
			Key:     strconv.FormatInt(id, 10),
			Payload: payload,
		}}, nil
	})
}

func (s *serv) ValidateCredentials(ctx context.Context, email, password string) (bool, int64, error) {
	u, hash, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return false, 0, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return false, 0, nil
	}
	return true, u.ID, nil
}
