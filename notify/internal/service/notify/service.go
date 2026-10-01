package notify

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"

	devicerepo "notify/internal/repository/device"
	"notify/internal/push"
)

type PresenceChecker interface {
	IsOnline(userID int64) (bool, error)
}

type RedisPresence struct {
	pool *redis.Pool
}

func NewRedisPresence(pool *redis.Pool) *RedisPresence {
	return &RedisPresence{pool: pool}
}

func (p *RedisPresence) IsOnline(userID int64) (bool, error) {
	if p == nil || p.pool == nil {
		return false, nil
	}
	c := p.pool.Get()
	defer c.Close()
	return redis.Bool(c.Do("EXISTS", fmt.Sprintf("presence:user:%d", userID)))
}

type Service struct {
	repo     *devicerepo.Repository
	push     push.Provider
	presence PresenceChecker
	dedup    *redis.Pool
}

func NewService(repo *devicerepo.Repository, p push.Provider, presence PresenceChecker, dedup *redis.Pool) *Service {
	return &Service{repo: repo, push: p, presence: presence, dedup: dedup}
}

func (s *Service) RegisterDevice(ctx context.Context, userID int64, platform, token string) error {
	return s.repo.Register(ctx, userID, platform, token)
}

func (s *Service) UnregisterDevice(ctx context.Context, userID int64, token string) error {
	return s.repo.Unregister(ctx, userID, token)
}

func (s *Service) NotifyUser(ctx context.Context, userID int64, title, body string) error {
	if s.presence != nil {
		online, err := s.presence.IsOnline(userID)
		if err == nil && online {
			return nil
		}
	}
	if s.seenRecently(userID, title, body) {
		return nil
	}
	devices, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, d := range devices {
		if err := s.push.Send(ctx, d, title, body); err != nil {
			if errors.Is(err, push.ErrInvalidToken) {
				_ = s.repo.Unregister(ctx, d.UserID, d.PushToken)
			}
		}
	}
	return nil
}

func (s *Service) NotifyChatMembers(ctx context.Context, memberIDs []int64, title, body string) error {
	for _, uid := range memberIDs {
		_ = s.NotifyUser(ctx, uid, title, body)
	}
	return nil
}

func (s *Service) NotifyAll(ctx context.Context, title, body string) error {
	devices, err := s.repo.ListAllTokens(ctx)
	if err != nil {
		return err
	}
	seenUsers := map[int64]struct{}{}
	for _, d := range devices {
		if _, ok := seenUsers[d.UserID]; ok {
			continue
		}
		seenUsers[d.UserID] = struct{}{}
		_ = s.NotifyUser(ctx, d.UserID, title, body)
	}
	return nil
}

func (s *Service) seenRecently(userID int64, title, body string) bool {
	if s.dedup == nil {
		return false
	}
	sum := sha1.Sum([]byte(fmt.Sprintf("%d:%s:%s", userID, title, body)))
	key := "notify:dedup:" + hex.EncodeToString(sum[:])
	c := s.dedup.Get()
	defer c.Close()
	ok, err := redis.String(c.Do("SET", key, "1", "EX", int((5*time.Minute).Seconds()), "NX"))
	if err != nil || ok != "OK" {
		return true
	}
	return false
}
