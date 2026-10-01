package cache

import "errors"

// ErrCacheMiss ключ отсутствует (аналог redis.ErrNil для read-through кэша).
var ErrCacheMiss = errors.New("cache miss")
