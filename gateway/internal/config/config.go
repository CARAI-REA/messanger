package config

import (
	"fmt"
	"os"

	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/joho/godotenv"

	"gateway/internal/config/env"
)

var appConfig *config

type config struct {
	App     AppEnvConfig
	Logger  LoggerConfig
	HTTP    HTTPConfig
	JWT     JWTConfig
	Metrics MetricsConfig
	Redis   RedisConfig
	Gateway GatewayConfig
}

func Load(path ...string) error {
	err := godotenv.Load(path...)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	appCfg, err := env.NewAppConfig()
	if err != nil {
		return err
	}
	loggerCfg, err := env.NewLoggerConfig()
	if err != nil {
		return err
	}
	httpCfg, err := env.NewHTTPConfig()
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
	redisCfg, err := env.NewRedisConfig()
	if err != nil {
		return err
	}
	gwCfg, err := env.NewGatewayConfig()
	if err != nil {
		return err
	}
	appConfig = &config{
		App: appCfg, Logger: loggerCfg, HTTP: httpCfg,
		JWT: jwtCfg, Metrics: metricsCfg, Redis: redisCfg, Gateway: gwCfg,
	}
	return appConfig.ValidateProduction()
}

func AppConfig() *config { return appConfig }

func (c *config) ValidateProduction() error {
	if c == nil || c.App == nil {
		return fmt.Errorf("config is not loaded")
	}
	if !prodguard.IsProduction(c.App.Env()) {
		return nil
	}
	origins := ""
	if c.Gateway != nil {
		origins = joinComma(c.Gateway.AllowedOrigins())
	}
	checks := []error{
		prodguard.ForbidWeakSecret("JWT_SECRET", c.JWT.AuthTokenSecretKey()),
		prodguard.ForbidWeakSecret("SERVICE_JWT_SECRET", c.JWT.ServiceTokenSecretKey()),
		prodguard.RequireNonEmpty("REDIS_PASSWORD", c.Redis.Password()),
		prodguard.ForbidWildcardOrigins("WS_ALLOWED_ORIGINS", origins),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
