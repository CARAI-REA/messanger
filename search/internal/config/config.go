package config

import (
	"fmt"
	"os"
	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/joho/godotenv"
	"search/internal/config/env"
)

var appConfig *config

type config struct {
	App AppEnvConfig
	Logger LoggerConfig
	GRPC GRPCConfig
	JWT JWTConfig
	Metrics MetricsConfig
	Kafka KafkaConfig
	OpenSearch OpenSearchConfig
}

func Load(path ...string) error {
	err := godotenv.Load(path...)
	if err != nil && !os.IsNotExist(err) { return err }
	appCfg, err := env.NewAppConfig(); if err != nil { return err }
	loggerCfg, err := env.NewLoggerConfig(); if err != nil { return err }
	grpcCfg, err := env.NewGRPCConfig(); if err != nil { return err }
	jwtCfg, err := env.NewJWTConfig(); if err != nil { return err }
	metricsCfg, err := env.NewMetricsConfig(); if err != nil { return err }
	kafkaCfg, err := env.NewKafkaConfig(); if err != nil { return err }
	osCfg, err := env.NewOpenSearchConfig(); if err != nil { return err }
	appConfig = &config{App: appCfg, Logger: loggerCfg, GRPC: grpcCfg, JWT: jwtCfg, Metrics: metricsCfg, Kafka: kafkaCfg, OpenSearch: osCfg}
	return appConfig.ValidateProduction()
}
func AppConfig() *config { return appConfig }
func (c *config) ValidateProduction() error {
	if c == nil || c.App == nil { return fmt.Errorf("config is not loaded") }
	if !prodguard.IsProduction(c.App.Env()) { return nil }
	return prodguard.ForbidWeakSecret("JWT_SECRET", c.JWT.AuthTokenSecretKey())
}
