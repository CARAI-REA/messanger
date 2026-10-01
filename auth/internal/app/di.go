package app

import (
	"context"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	authv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/auth/v1"
	"github.com/gomodule/redigo/redis"

	authv1api "auth/internal/api/auth/v1"
	"auth/internal/config"
	"auth/internal/rediswire"
	authrepo "auth/internal/repository/auth"
	authsvc "auth/internal/service/auth"
)

type diContainer struct {
	pool *redis.Pool
	svc  *authsvc.Service
	api  authv1.AuthServiceServer
}

func NewDiContainer() *diContainer { return &diContainer{} }

func (d *diContainer) Redis() *redis.Pool {
	if d.pool != nil {
		return d.pool
	}
	cfg := config.AppConfig().Redis
	d.pool = rediswire.NewPool(cfg.Address(), cfg.Password, cfg.MaxIdle, cfg.IdleTimeout, cfg.ConnTimeout)
	closer.AddNamed("redis", func(context.Context) error {
		return d.pool.Close()
	})
	return d.pool
}

func (d *diContainer) AuthService() *authsvc.Service {
	if d.svc != nil {
		return d.svc
	}
	cfg := config.AppConfig()
	repo := authrepo.NewRepository(d.Redis(), cfg.Redis.LoginMaxAttempts, cfg.Redis.LoginWindow, cfg.JWT.RefreshTTL())
	svcTok := jwtTokens.NewServiceJWTService(cfg.JWT)
	svc, err := authsvc.NewService(
		repo,
		authsvc.UserDialConfig{
			Address:    cfg.User.Address,
			TLS:        cfg.User.TLS,
			CAFile:     cfg.User.CAFile,
			ServerName: cfg.User.ServerName,
		},
		svcTok,
		cfg.JWT.AuthTokenSecretKey(),
		cfg.JWT.RefreshTokenSecretKey(),
		cfg.JWT.AccessTTL(),
		cfg.JWT.RefreshTTL(),
	)
	if err != nil {
		panic(err)
	}
	closer.AddNamed("auth service", func(context.Context) error { return svc.Close() })
	d.svc = svc
	return d.svc
}

func (d *diContainer) AuthAPI() authv1.AuthServiceServer {
	if d.api != nil {
		return d.api
	}
	d.api = authv1api.New(d.AuthService())
	return d.api
}
