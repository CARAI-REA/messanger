package auth

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Repository struct {
	pool         *redis.Pool
	maxAttempts  int
	window       time.Duration
	refreshTTL   time.Duration
}

func NewRepository(pool *redis.Pool, maxAttempts int, window, refreshTTL time.Duration) *Repository {
	return &Repository{pool: pool, maxAttempts: maxAttempts, window: window, refreshTTL: refreshTTL}
}

func (r *Repository) IncrLoginAttempts(email string) (int, error) {
	c := r.pool.Get()
	defer c.Close()
	key := "login_attempts:" + email
	n, err := redis.Int(c.Do("INCR", key))
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_, _ = c.Do("EXPIRE", key, int(r.window.Seconds()))
	}
	return n, nil
}

func (r *Repository) ResetLoginAttempts(email string) error {
	c := r.pool.Get()
	defer c.Close()
	_, err := c.Do("DEL", "login_attempts:"+email)
	return err
}

func (r *Repository) IsLocked(email string) (bool, error) {
	c := r.pool.Get()
	defer c.Close()
	n, err := redis.Int(c.Do("GET", "login_attempts:"+email))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n >= r.maxAttempts, nil
}

func (r *Repository) IncrRefreshAttempts(userID int64) (int, error) {
	c := r.pool.Get()
	defer c.Close()
	key := fmt.Sprintf("refresh_attempts:%d", userID)
	n, err := redis.Int(c.Do("INCR", key))
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_, _ = c.Do("EXPIRE", key, int(r.window.Seconds()))
	}
	return n, nil
}

func (r *Repository) IsRefreshLocked(userID int64) (bool, error) {
	c := r.pool.Get()
	defer c.Close()
	n, err := redis.Int(c.Do("GET", fmt.Sprintf("refresh_attempts:%d", userID)))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n >= r.maxAttempts, nil
}

func (r *Repository) StoreRefresh(jti string, userID int64) error {
	c := r.pool.Get()
	defer c.Close()
	key := "refresh:" + jti
	userKey := fmt.Sprintf("refresh_user:%d", userID)
	if _, err := c.Do("SETEX", key, int(r.refreshTTL.Seconds()), userID); err != nil {
		return err
	}
	_, err := c.Do("SADD", userKey, jti)
	return err
}

func (r *Repository) GetRefreshUserID(jti string) (int64, error) {
	c := r.pool.Get()
	defer c.Close()
	return redis.Int64(c.Do("GET", "refresh:"+jti))
}

func (r *Repository) RevokeRefresh(jti string, userID int64) error {
	c := r.pool.Get()
	defer c.Close()
	_, _ = c.Do("DEL", "refresh:"+jti)
	_, err := c.Do("SREM", fmt.Sprintf("refresh_user:%d", userID), jti)
	return err
}

// RotateRefresh atomically deletes oldJTI and stores newJTI for the same user.
// Returns false if oldJTI was already revoked (double-refresh race).
func (r *Repository) RotateRefresh(oldJTI, newJTI string, userID int64) (bool, error) {
	c := r.pool.Get()
	defer c.Close()
	script := `
local oldKey = KEYS[1]
local newKey = KEYS[2]
local userKey = KEYS[3]
local uid = ARGV[1]
local ttl = tonumber(ARGV[2])
local newJti = ARGV[3]
local oldJti = ARGV[4]
local cur = redis.call('GET', oldKey)
if (not cur) or (cur ~= uid) then
  return 0
end
redis.call('DEL', oldKey)
redis.call('SREM', userKey, oldJti)
redis.call('SETEX', newKey, ttl, uid)
redis.call('SADD', userKey, newJti)
return 1
`
	oldKey := "refresh:" + oldJTI
	newKey := "refresh:" + newJTI
	userKey := fmt.Sprintf("refresh_user:%d", userID)
	n, err := redis.Int(c.Do("EVAL", script, 3, oldKey, newKey, userKey,
		fmt.Sprintf("%d", userID), int(r.refreshTTL.Seconds()), newJTI, oldJTI))
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
