package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/otelx"
	"go.uber.org/zap"

	"media/internal/app"
	"media/internal/config"
)

const configPath = "./../deploy/compose/media/.env"

func main() {
	if err := config.Load(configPath); err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
	appCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	defer func() {
		ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = closer.CloseAll(ctx)
	}()
	closer.Configure(syscall.SIGINT, syscall.SIGTERM)
	if sh, err := otelx.InitTracer(appCtx, "media"); err == nil {
		closer.AddNamed("otel", sh)
	}
	a, err := app.New(appCtx)
	if err != nil {
		logger.Error(appCtx, "app create", zap.Error(err))
		return
	}
	if err := a.Run(appCtx); err != nil {
		logger.Error(appCtx, "app run", zap.Error(err))
	}
}
