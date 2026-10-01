package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gateway/internal/config"
)

type App struct {
	di            *diContainer
	httpServer    *http.Server
	metricsServer *http.Server
}

func New(ctx context.Context) (*App, error) {
	a := &App{di: NewDiContainer()}
	if err := logger.Init(config.AppConfig().Logger.Level(), config.AppConfig().Logger.AsJson()); err != nil {
		return nil, err
	}
	closer.SetLogger(logger.Logger())

	mux := http.NewServeMux()
	mux.Handle("/ws", a.di.WSHandler())
	mux.Handle("/v1/ws", a.di.WSHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if a.di.WSHandler().IsDraining() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("draining"))
			return
		}
		if err := a.di.Presence().Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("redis unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/internal/drain", a.handleDrain)
	a.httpServer = &http.Server{
		Addr:              config.AppConfig().HTTP.Address(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	closer.AddNamed("http", func(ctx context.Context) error {
		handler := a.di.WSHandler()
		handler.BeginDrain()
		handler.WaitEmpty(config.AppConfig().Gateway.DrainDuration())
		return a.httpServer.Shutdown(ctx)
	})

	mmux := http.NewServeMux()
	mmux.Handle("/metrics", promhttp.Handler())
	addr := config.AppConfig().Metrics.Address()
	if prodguard.IsProduction(config.AppConfig().App.Env()) {
		_, port, err := net.SplitHostPort(addr)
		if err == nil {
			addr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	a.metricsServer = &http.Server{Addr: addr, Handler: mmux, ReadHeaderTimeout: 5 * time.Second}
	closer.AddNamed("metrics", func(ctx context.Context) error { return a.metricsServer.Shutdown(ctx) })
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	a.di.StartRealtimeSubscriber(ctx)
	errCh := make(chan error, 2)
	go func() {
		logger.Info(ctx, fmt.Sprintf("metrics on %s", config.AppConfig().Metrics.Address()))
		if err := a.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		logger.Info(ctx, fmt.Sprintf("gateway HTTP/WS on %s", config.AppConfig().HTTP.Address()))
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func (a *App) handleDrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "drain only from localhost", http.StatusForbidden)
		return
	}

	handler := a.di.WSHandler()
	if !handler.BeginDrain() {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("already draining"))
		return
	}

	waitFor := config.AppConfig().Gateway.DrainDuration()
	logger.Info(r.Context(), fmt.Sprintf("gateway drain started; waiting %s", waitFor))
	handler.WaitEmpty(waitFor)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("drained"))
}
