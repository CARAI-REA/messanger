package rediswire

import (
	"time"
	"github.com/gomodule/redigo/redis"
)

func NewPool(addr, password string, maxIdle int, idleTimeout, connectTimeout time.Duration) *redis.Pool {
	return &redis.Pool{
		MaxIdle: maxIdle,
		IdleTimeout: idleTimeout,
		Dial: func() (redis.Conn, error) {
			opts := []redis.DialOption{redis.DialConnectTimeout(connectTimeout)}
			if password != "" {
				opts = append(opts, redis.DialPassword(password))
			}
			return redis.Dial("tcp", addr, opts...)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute { return nil }
			_, err := c.Do("PING")
			return err
		},
	}
}
