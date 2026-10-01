package config

import (
	"fmt"
	"os"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/joho/godotenv"

	"auth/internal/config/env"
)

var appConfig *config

type config struct {
	App     *env.AppConfig
	Logger  *env.LoggerConfig
	GRPC    *env.GRPCConfig
	Redis   *env.RedisConfig
	JWT     *env.JWTConfig
	Metrics *env.MetricsConfig
	User    *env.UserClientConfig
}

func Load(path ...string) error {
	err := godotenv.Load(path...)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	app, err := env.NewAppConfig()
	if err != nil {
		return err
	}
	loggerCfg, err := env.NewLoggerConfig()
	if err != nil {
		return err
	}
	grpcCfg, err := env.NewGRPCConfig()
	if err != nil {
		return err
	}
	redisCfg, err := env.NewRedisConfig()
	if err != nil {
		return err
	}
	jwtCfg, err := env.NewJWTConfig()
	if err != nil {
		return err
	}
	metricsCfg, err := env.NewMetricsConfig()
	if err != nil {
		return err
	}
	userCfg, err := env.NewUserClientConfig()
	if err != nil {
		return err
	}
	appConfig = &config{
		App: app, Logger: loggerCfg, GRPC: grpcCfg,
		Redis: redisCfg, JWT: jwtCfg, Metrics: metricsCfg, User: userCfg,
	}
	return appConfig.ValidateProduction()
}

func AppConfig() *config { return appConfig }

func (c *config) AccessTTL() time.Duration  { return c.JWT.AccessTTL() }
func (c *config) RefreshTTL() time.Duration { return c.JWT.RefreshTTL() }

func (c *config) ValidateProduction() error {
	if c == nil || c.App == nil {
		return fmt.Errorf("config is not loaded")
	}
	if !prodguard.IsProduction(c.App.Env()) {
		return nil
	}
	checks := []error{
		prodguard.ForbidWeakSecret("JWT_SECRET", c.JWT.AuthTokenSecretKey()),
		prodguard.ForbidWeakSecret("REFRESH_TOKEN_SECRET", c.JWT.RefreshTokenSecretKey()),
		prodguard.ForbidWeakSecret("SERVICE_JWT_SECRET", c.JWT.ServiceTokenSecretKey()),
		prodguard.RequireNonEmpty("REDIS_PASSWORD", c.Redis.Password),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}
