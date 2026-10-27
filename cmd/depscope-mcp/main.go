// Command depscope-mcp 启动 DepScope 的 Model Context Protocol (MCP) 本地服务。
// 仅允许通过 stdio 进行协议通信，所有日志输出均定向到 stderr，确保 stdout 纯净。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/mcp"
)

var version = "0.4.0-dev"

type sliceFlag []string

func (f *sliceFlag) String() string {
	return strings.Join(*f, ", ")
}

func (f *sliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	var allowedPaths sliceFlag
	flag.Var(&allowedPaths, "allowed-path", "Permitted directory path for project scans (can be specified multiple times)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "depscope-mcp %s\n", version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, allowedPaths, logger); err != nil {
		logger.Error("depscope-mcp server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, allowedPaths []string, logger *slog.Logger) error {
	service, err := app.NewLiveService(app.LiveOptions{})
	if err != nil {
		return fmt.Errorf("create scan service: %w", err)
	}

	server, err := mcp.NewServer(mcp.ServerOptions{
		Service:      service,
		AllowedPaths: allowedPaths,
		Logger:       logger,
		Version:      version,
	})
	if err != nil {
		return fmt.Errorf("create mcp server: %w", err)
	}

	return server.ServeStdio(ctx)
}
