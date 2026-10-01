package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/grpc/health"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/grpctls"
	"github.com/CARAI-REA/messanger/platform/pkg/grpcmw"
	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	chatv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/chat/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"chat/internal/config"
	"chat/internal/interceptor"
)

type App struct {
	di            *diContainer
	grpcServer    *grpc.Server
	listener      net.Listener
	metricsServer *http.Server
}

func New(ctx context.Context) (*App, error) {
	a := &App{}
	if err := a.initDeps(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	go a.di.OutboxWorker(ctx).Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		logger.Info(ctx, fmt.Sprintf("metrics listening on %s", config.AppConfig().Metrics.Address()))
		if err := a.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		logger.Info(ctx, fmt.Sprintf("gRPC ChatService listening on %s", config.AppConfig().GRPC.Address()))
		if err := a.grpcServer.Serve(a.listener); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) initDeps(ctx context.Context) error {
	for _, f := range []func(context.Context) error{
		a.initDi, a.initLogger, a.initCloser, a.initListener, a.initGRPCServer, a.initMetricsServer,
	} {
		if err := f(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) initDi(_ context.Context) error {
	a.di = NewDiContainer()
	return nil
}

func (a *App) initLogger(_ context.Context) error {
	return logger.Init(config.AppConfig().Logger.Level(), config.AppConfig().Logger.AsJson())
}

func (a *App) initCloser(_ context.Context) error {
	closer.SetLogger(logger.Logger())
	return nil
}

func (a *App) initListener(_ context.Context) error {
	listener, err := net.Listen("tcp", config.AppConfig().GRPC.Address())
	if err != nil {
		return err
	}
	closer.AddNamed("TCP listener", func(ctx context.Context) error {
		lerr := listener.Close()
		if lerr != nil && !errors.Is(lerr, net.ErrClosed) {
			return lerr
		}
		return nil
	})
	a.listener = listener
	return nil
}

func (a *App) initGRPCServer(ctx context.Context) error {
	tlsOpts, err := grpctls.ServerOptions(
		config.AppConfig().GRPC.TLSCertFile(),
		config.AppConfig().GRPC.TLSKeyFile(),
		config.AppConfig().GRPC.TLSClientCA(),
	)
	if err != nil {
		return err
	}
	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			interceptor.AuthInterceptor(a.di.AccessVerifier(), a.di.ServiceVerifier()),
			interceptor.LoggerInterceptor(),
			grpcmw.RateLimitInterceptor(config.AppConfig().App.GRPCMaxRPS()),
		),
	}
	opts = append(opts, tlsOpts...)
	a.grpcServer = grpc.NewServer(opts...)
	closer.AddNamed("gRPC server", func(ctx context.Context) error {
		a.grpcServer.GracefulStop()
		return nil
	})
	reflection.Register(a.grpcServer)
	health.RegisterService(a.grpcServer)
	chatv1.RegisterChatServiceServer(a.grpcServer, a.di.ChatAPI(ctx))
	return nil
}

func (a *App) initMetricsServer(_ context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	addr := config.AppConfig().Metrics.Address()
	if prodguard.IsProduction(config.AppConfig().App.Env()) {
		_, port, err := net.SplitHostPort(addr)
		if err == nil {
			addr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	a.metricsServer = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	closer.AddNamed("metrics HTTP server", func(ctx context.Context) error {
		return a.metricsServer.Shutdown(ctx)
	})
	return nil
}
