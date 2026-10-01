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
	notifyv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/notify/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"notify/internal/config"
	"notify/internal/interceptor"
)

type App struct {
	di            *diContainer
	grpcServer    *grpc.Server
	listener      net.Listener
	metricsServer *http.Server
}

func New(ctx context.Context) (*App, error) {
	a := &App{di: NewDiContainer()}
	if err := logger.Init(config.AppConfig().Logger.Level(), config.AppConfig().Logger.AsJson()); err != nil {
		return nil, err
	}
	closer.SetLogger(logger.Logger())
	listener, err := net.Listen("tcp", config.AppConfig().GRPC.Address())
	if err != nil {
		return nil, err
	}
	a.listener = listener
	closer.AddNamed("listener", func(context.Context) error {
		err := listener.Close()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	})
	tlsOpts, err := grpctls.ServerOptions(
		config.AppConfig().GRPC.TLSCertFile(),
		config.AppConfig().GRPC.TLSKeyFile(),
		config.AppConfig().GRPC.TLSClientCA(),
	)
	if err != nil {
		return nil, err
	}
	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			interceptor.AuthInterceptor(a.di.AccessVerifier()),
			interceptor.LoggerInterceptor(),
			grpcmw.RateLimitInterceptor(config.AppConfig().App.GRPCMaxRPS()),
		),
	}
	opts = append(opts, tlsOpts...)
	a.grpcServer = grpc.NewServer(opts...)
	reflection.Register(a.grpcServer)
	health.RegisterService(a.grpcServer)
	notifyv1.RegisterNotifyServiceServer(a.grpcServer, a.di.API(ctx))
	closer.AddNamed("grpc", func(context.Context) error { a.grpcServer.GracefulStop(); return nil })

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
	closer.AddNamed("metrics", func(ctx context.Context) error { return a.metricsServer.Shutdown(ctx) })
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	go a.di.Consumer(ctx).Run(ctx)
	errCh := make(chan error, 1)
	go func() {
		logger.Info(ctx, fmt.Sprintf("metrics on %s", config.AppConfig().Metrics.Address()))
		if err := a.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		logger.Info(ctx, fmt.Sprintf("notify gRPC on %s", config.AppConfig().GRPC.Address()))
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
