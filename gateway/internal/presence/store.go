package presence

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Store struct {
	pool        *redis.Pool
	presenceTTL time.Duration
	typingTTL   time.Duration
}

func New(pool *redis.Pool, presenceTTL, typingTTL time.Duration) *Store {
	return &Store{pool: pool, presenceTTL: presenceTTL, typingTTL: typingTTL}
}

func presenceKey(userID int64) string {
	return fmt.Sprintf("presence:user:%d", userID)
}

func typingKey(chatID, userID int64) string {
	return fmt.Sprintf("typing:chat:%d:user:%d", chatID, userID)
}

func (s *Store) SetOnline(userID int64) error {
	conn := s.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SET", presenceKey(userID), "1", "EX", int(s.presenceTTL.Seconds()))
	return err
}

func (s *Store) Heartbeat(userID int64) error {
	return s.SetOnline(userID)
}

func (s *Store) SetOffline(userID int64) error {
	conn := s.pool.Get()
	defer conn.Close()
	_, err := conn.Do("DEL", presenceKey(userID))
	return err
}

func (s *Store) SetTyping(chatID, userID int64) error {
	conn := s.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SET", typingKey(chatID, userID), "1", "EX", int(s.typingTTL.Seconds()))
	return err
}

func (s *Store) IsOnline(userID int64) (bool, error) {
	conn := s.pool.Get()
	defer conn.Close()
	return redis.Bool(conn.Do("EXISTS", presenceKey(userID)))
}

func (s *Store) Ping() error {
	conn := s.pool.Get()
	defer conn.Close()
	_, err := conn.Do("PING")
	return err
}

func ParseUserID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
