package app

import (
	"context"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	chatv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/chat/v1"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	chatv1api "chat/internal/api/chat/v1"
	"chat/internal/config"
	"chat/internal/kafka"
	"chat/internal/rediswire"
	chatrepo "chat/internal/repository/chat"
	outboxrepo "chat/internal/repository/outbox"
	"chat/internal/service"
	chatsvc "chat/internal/service/chat"
)

type diContainer struct {
	postgresClient  *pgxpool.Pool
	redisPool       *redis.Pool
	accessVerifier  tokens.AccessTokenVerifier
	serviceVerifier tokens.ServiceTokenVerifier
	svc             service.ChatService
	api             chatv1.ChatServiceServer
	outboxWorker    *kafka.OutboxWorker
}

func NewDiContainer() *diContainer {
	return &diContainer{}
}

func (d *diContainer) PostgresClient(ctx context.Context) *pgxpool.Pool {
	if d.postgresClient != nil {
		return d.postgresClient
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
	closer.AddNamed("postgres", func(context.Context) error {
		pool.Close()
		return nil
	})
	d.postgresClient = pool
	return d.postgresClient
}

func (d *diContainer) RedisPool() *redis.Pool {
	if d.redisPool != nil {
		return d.redisPool
	}
	cfg := config.AppConfig().Redis
	d.redisPool = rediswire.NewPool(cfg.Address(), cfg.Password(), cfg.MaxIdle(), cfg.IdleTimeout(), cfg.ConnTimeout())
	closer.AddNamed("redis", func(context.Context) error {
		return d.redisPool.Close()
	})
	return d.redisPool
}

func (d *diContainer) ChatService(ctx context.Context) service.ChatService {
	if d.svc != nil {
		return d.svc
	}
	pool := d.PostgresClient(ctx)
	d.svc = chatsvc.NewService(chatrepo.NewRepository(pool), outboxrepo.NewRepository(pool))
	return d.svc
}

func (d *diContainer) ChatAPI(ctx context.Context) chatv1.ChatServiceServer {
	if d.api != nil {
		return d.api
	}
	d.api = chatv1api.NewImplementation(d.ChatService(ctx))
	return d.api
}

func (d *diContainer) AccessVerifier() tokens.AccessTokenVerifier {
	if d.accessVerifier != nil {
		return d.accessVerifier
	}
	d.accessVerifier = jwtTokens.NewAccessJWTVerifier(config.AppConfig().JWT)
	return d.accessVerifier
}

func (d *diContainer) ServiceVerifier() tokens.ServiceTokenVerifier {
	if d.serviceVerifier != nil {
		return d.serviceVerifier
	}
	d.serviceVerifier = jwtTokens.NewServiceJWTService(config.AppConfig().JWT)
	return d.serviceVerifier
}

func (d *diContainer) OutboxWorker(ctx context.Context) *kafka.OutboxWorker {
	if d.outboxWorker != nil {
		return d.outboxWorker
	}
	pool := d.PostgresClient(ctx)
	orepo := outboxrepo.NewRepository(pool)
	w, err := kafka.NewOutboxWorker(
		orepo,
		config.AppConfig().Kafka.Brokers(),
		d.RedisPool(),
		config.AppConfig().Realtime.Channel(),
	)
	if err != nil {
		logger.Error(ctx, "kafka producer init failed")
		panic(err)
	}
	closer.AddNamed("outbox worker", func(context.Context) error { return w.Close() })
	d.outboxWorker = w
	return d.outboxWorker
}
