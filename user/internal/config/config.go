package config

import (
	"fmt"
	"os"

	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/joho/godotenv"

	"user/internal/config/env"
)

var appConfig *config

type config struct {
	App      AppEnvConfig
	Logger   LoggerConfig
	GRPC     GRPCConfig
	Postgres PostgresConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
	Metrics  MetricsConfig
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
	grpcCfg, err := env.NewGRPCConfig()
	if err != nil {
		return err
	}
	postgresCfg, err := env.NewPostgresConfig()
	if err != nil {
		return err
	}
	kafkaCfg, err := env.NewKafkaConfig()
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
	appConfig = &config{
		App: appCfg, Logger: loggerCfg, GRPC: grpcCfg,
		Postgres: postgresCfg, Kafka: kafkaCfg, JWT: jwtCfg, Metrics: metricsCfg,
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
	checks := []error{
		prodguard.ForbidWeakSecret("JWT_SECRET", c.JWT.AuthTokenSecretKey()),
		prodguard.ForbidWeakSecret("SERVICE_JWT_SECRET", c.JWT.ServiceTokenSecretKey()),
		prodguard.ForbidDisabledSSL("POSTGRES_SSL_MODE", c.Postgres.SSLMode()),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}
