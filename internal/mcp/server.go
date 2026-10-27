// Package mcp 基于官方 Go MCP SDK 实现 DepScope 的本地 Model Context Protocol 服务端。
// 遵循最小权限原则：仅允许对受限目录与依赖包执行只读安全扫描，协议交互仅使用 stdio，
// 所有诊断日志与错误均定向至 stderr，确保 stdout 保持干净的 JSON-RPC 协议流。
package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/janghotan/depscope/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerOptions 配置 DepScope MCP 实例。
type ServerOptions struct {
	Service      app.Service
	AllowedPaths []string
	Logger       *slog.Logger
	LogOutput    io.Writer
	Version      string
}

// Server 封装基于官方 Go SDK 的 MCP 服务实例。
type Server struct {
	mcpServer *mcp.Server
	registry  *toolHandlerRegistry
	validator *PathValidator
	cache     *ScanCache
	logger    *slog.Logger
}

// NewServer 初始化 MCP 服务器并注册所有受支持的安全扫描工具。
func NewServer(opts ServerOptions) (*Server, error) {
	if opts.Service == nil {
		return nil, errors.New("mcp server requires a scan service")
	}

	validator, err := NewPathValidator(opts.AllowedPaths)
	if err != nil {
		return nil, fmt.Errorf("initialize path validator: %w", err)
	}

	version := opts.Version
	if version == "" {
		version = "0.4.0-dev"
	}

	logger := opts.Logger
	if logger == nil {
		out := opts.LogOutput
		if out == nil {
			out = os.Stderr
		}
		logger = slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	cache := NewScanCache(50)
	registry := newToolHandlerRegistry(opts.Service, validator, cache)

	impl := &mcp.Implementation{
		Name:    "depscope",
		Version: version,
	}

	serverOpts := &mcp.ServerOptions{
		Instructions: "DepScope is a local software supply chain security scanner. " +
			"Use scan_project to audit supported lockfiles (npm package-lock.json v2/v3, python requirements.txt) in permitted directories. " +
			"Use scan_package to inspect packages and versions against OSV and registries. " +
			"Use get_finding_details to inspect finding evidence and dependency paths. " +
			"Use get_scan_summary to retrieve severity totals and resolution states.",
		Logger: logger,
	}

	mcpServer := mcp.NewServer(impl, serverOpts)

	// 注册四大核心工具
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "scan_project",
		Description: "Scan a permitted project directory using DepScope's project scanner to inspect dependencies, vulnerabilities, and risks.",
	}, registry.scanProject)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "scan_package",
		Description: "Scan an individual package and version using DepScope's security engine to inspect advisories, risk assessment, and metadata.",
	}, registry.scanPackage)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_finding_details",
		Description: "Retrieve comprehensive details, evidence, risk assessment, advisory URLs, and dependency context for a specific finding.",
	}, registry.getFindingDetails)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "get_scan_summary",
		Description: "Return structured scan totals, severity counts, and overall status for a scan or target.",
	}, registry.getScanSummary)

	return &Server{
		mcpServer: mcpServer,
		registry:  registry,
		validator: validator,
		cache:     cache,
		logger:    logger,
	}, nil
}

// ServeStdio 在标准输入输出 (stdio) 上运行 MCP 会话。
func (s *Server) ServeStdio(ctx context.Context) error {
	s.logger.Info("starting DepScope MCP server on stdio transport",
		"allowedRoots", s.validator.AllowedRoots(),
	)
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// ServeTransport 在自定义 MCP Transport 上运行会话（便于内存测试或进程间管道）。
func (s *Server) ServeTransport(ctx context.Context, transport mcp.Transport) error {
	return s.mcpServer.Run(ctx, transport)
}

// MCPServer 返回底层 MCP Server 指针。
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}

// Cache 返回会话内的扫描缓存。
func (s *Server) Cache() *ScanCache {
	return s.cache
}

// Validator 返回路径校验器。
func (s *Server) Validator() *PathValidator {
	return s.validator
}
