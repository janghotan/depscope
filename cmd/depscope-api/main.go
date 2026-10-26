// Command depscope-api 启动 DepScope 的 HTTP API。
// 该进程只装配共享扫描服务；扫描规则与报告组装不在本命令内重复实现。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"depscope/internal/api"
	"depscope/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("depscope api stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	config, err := api.LoadConfigFromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("load api config: %w", err)
	}

	service, err := app.NewLiveService(app.LiveOptions{})
	if err != nil {
		return fmt.Errorf("create scan service: %w", err)
	}

	server, err := api.NewServer(api.Options{
		Config:  config,
		Service: service,
		Logger:  logger,
	})
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              config.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      config.RequestTimeout + 15*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.ListenAndServe()
	}()
	logger.Info("DepScope API listening",
		"addr", config.Addr,
		"maxConcurrency", config.MaxConcurrency,
		"requestTimeout", config.RequestTimeout.String(),
	)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		_ = httpServer.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("DepScope API stopped")
	return nil
}
