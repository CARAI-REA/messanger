package app

import (
	"context"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	mediav1 "github.com/CARAI-REA/messanger/shared/pkg/proto/media/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	mediav1api "media/internal/api/media/v1"
	"media/internal/config"
	mediarepo "media/internal/repository/media"
	s3store "media/internal/s3"
	mediasvc "media/internal/service/media"
)

type diContainer struct {
	pool           *pgxpool.Pool
	s3             *s3store.Client
	svc            *mediasvc.Service
	api            mediav1.MediaServiceServer
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

func (d *diContainer) S3() *s3store.Client {
	if d.s3 != nil {
		return d.s3
	}
	c, err := s3store.New(config.AppConfig().S3)
	if err != nil {
		panic(err)
	}
	d.s3 = c
	return d.s3
}

func (d *diContainer) Service(ctx context.Context) *mediasvc.Service {
	if d.svc != nil {
		return d.svc
	}
	d.svc = mediasvc.NewService(mediarepo.NewRepository(d.Postgres(ctx)), d.S3())
	return d.svc
}

func (d *diContainer) API(ctx context.Context) mediav1.MediaServiceServer {
	if d.api != nil {
		return d.api
	}
	d.api = mediav1api.New(d.Service(ctx))
	return d.api
}

func (d *diContainer) AccessVerifier() tokens.AccessTokenVerifier {
	if d.accessVerifier != nil {
		return d.accessVerifier
	}
	d.accessVerifier = jwtTokens.NewAccessJWTVerifier(config.AppConfig().JWT)
	return d.accessVerifier
}
