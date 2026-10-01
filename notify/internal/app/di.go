package app

import (
	"context"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	notifyv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/notify/v1"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	notifyv1api "notify/internal/api/notify/v1"
	"notify/internal/config"
	"notify/internal/kafka"
	"notify/internal/push"
	devicerepo "notify/internal/repository/device"
	notifysvc "notify/internal/service/notify"
)

type diContainer struct {
	pool           *pgxpool.Pool
	redis          *redis.Pool
	svc            *notifysvc.Service
	api            notifyv1.NotifyServiceServer
	consumer       *kafka.Consumer
	accessVerifier tokens.AccessTokenVerifier
}

func NewDiContainer() *diContainer { return &diContainer{} }

func (d *diContainer) Postgres(ctx context.Context) *pgxpool.Pool {
	if d.pool != nil {
		return d.pool
	}
	pool, err := pgxpool.New(ctx, config.AppConfig().Postgres.URI())
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	if dir := config.AppConfig().Postgres.MigrationDir(); dir != "" {
		if err := goose.Up(stdlib.OpenDBFromPool(pool), dir); err != nil {
			panic(err)
		}
	}
	closer.AddNamed("postgres", func(context.Context) error { pool.Close(); return nil })
	d.pool = pool
	return d.pool
}

func (d *diContainer) Redis() *redis.Pool {
	if d.redis != nil {
		return d.redis
	}
	cfg := config.AppConfig().Redis
	if cfg == nil || !cfg.Enabled() {
		return nil
	}
	d.redis = &redis.Pool{
		MaxIdle:     cfg.MaxIdle(),
		IdleTimeout: cfg.IdleTimeout(),
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", cfg.Address(),
				redis.DialPassword(cfg.Password()),
				redis.DialConnectTimeout(cfg.ConnTimeout()),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	closer.AddNamed("redis", func(context.Context) error { return d.redis.Close() })
	return d.redis
}

func (d *diContainer) Service(ctx context.Context) *notifysvc.Service {
	if d.svc != nil {
		return d.svc
	}
	provider, err := push.New(config.AppConfig().Push.Provider(), config.AppConfig().Push.CredentialsFile())
	if err != nil {
		panic(err)
	}
	rpool := d.Redis()
	var presence notifysvc.PresenceChecker
	if rpool != nil {
		presence = notifysvc.NewRedisPresence(rpool)
	}
	d.svc = notifysvc.NewService(devicerepo.NewRepository(d.Postgres(ctx)), provider, presence, rpool)
	return d.svc
}

func (d *diContainer) API(ctx context.Context) notifyv1.NotifyServiceServer {
	if d.api != nil {
		return d.api
	}
	d.api = notifyv1api.New(d.Service(ctx))
	return d.api
}

func (d *diContainer) AccessVerifier() tokens.AccessTokenVerifier {
	if d.accessVerifier != nil {
		return d.accessVerifier
	}
	d.accessVerifier = jwtTokens.NewAccessJWTVerifier(config.AppConfig().JWT)
	return d.accessVerifier
}

func (d *diContainer) Consumer(ctx context.Context) *kafka.Consumer {
	if d.consumer != nil {
		return d.consumer
	}
	cfg := config.AppConfig().Kafka
	c, err := kafka.NewConsumer(cfg.Brokers(), cfg.GroupID(), []string{cfg.ChatEventsTopic()}, d.Service(ctx))
	if err != nil {
		panic(err)
	}
	closer.AddNamed("kafka consumer", func(context.Context) error { return c.Close() })
	d.consumer = c
	return d.consumer
}
