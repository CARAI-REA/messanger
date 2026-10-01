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

func (s *Store) AreOnline(userIDs []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	conn := s.pool.Get()
	defer conn.Close()
	args := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		args = append(args, presenceKey(id))
	}
	vals, err := redis.Ints(conn.Do("EXISTS", args...))
	if err != nil {
		// fallback per-key for older redis EXISTS multi semantics
		for _, id := range userIDs {
			ok, e := s.IsOnline(id)
			if e != nil {
				return nil, e
			}
			out[id] = ok
		}
		return out, nil
	}
	// Redis EXISTS with multiple keys returns count of existing keys, not per-key.
	// So always do pipeline GET/EXISTS per key.
	_ = vals
	for _, id := range userIDs {
		ok, e := redis.Bool(conn.Do("EXISTS", presenceKey(id)))
		if e != nil {
			return nil, e
		}
		out[id] = ok
	}
	return out, nil
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
