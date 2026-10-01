package app

import (
	"context"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	userv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/user/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	userv1api "user/internal/api/user/v1"
	"user/internal/config"
	"user/internal/kafka"
	outboxrepo "user/internal/repository/outbox"
	userrepo "user/internal/repository/user"
	"user/internal/service"
	usersvc "user/internal/service/user"
)

type diContainer struct {
	postgresClient  *pgxpool.Pool
	userRepository  interface{ /* filled lazily */ }
	userRepo        service.UserService
	userAPI         userv1.UserServiceServer
	accessVerifier  tokens.AccessTokenVerifier
	serviceVerifier tokens.ServiceTokenVerifier
	outboxWorker    *kafka.OutboxWorker
	svc             service.UserService
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

func (d *diContainer) UserService(ctx context.Context) service.UserService {
	if d.svc != nil {
		return d.svc
	}
	pool := d.PostgresClient(ctx)
	urepo := userrepo.NewRepository(pool)
	orepo := outboxrepo.NewRepository(pool)
	d.svc = usersvc.NewService(urepo, orepo)
	return d.svc
}

func (d *diContainer) UserAPI(ctx context.Context) userv1.UserServiceServer {
	if d.userAPI != nil {
		return d.userAPI
	}
	d.userAPI = userv1api.NewImplementation(d.UserService(ctx))
	return d.userAPI
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
	w, err := kafka.NewOutboxWorker(orepo, config.AppConfig().Kafka.Brokers())
	if err != nil {
		logger.Error(ctx, "kafka producer init failed - outbox disabled")
		panic(err)
	}
	closer.AddNamed("outbox worker", func(context.Context) error { return w.Close() })
	d.outboxWorker = w
	return d.outboxWorker
}
