// Package api 暴露共享扫描引擎的 HTTP 接口。
// 该包只负责传输、校验与进程内保护；扫描、风险判断与报告组装仍由 app.Service 完成。
package api

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// 环境变量名称；公开文档必须与这些常量保持一致。
const (
	EnvAddr            = "DEPSCOPE_API_ADDR"
	EnvRequestTimeout  = "DEPSCOPE_API_REQUEST_TIMEOUT"
	EnvMaxBodyBytes    = "DEPSCOPE_API_MAX_BODY_BYTES"
	EnvMaxConcurrency  = "DEPSCOPE_API_MAX_CONCURRENCY"
	EnvRatePerMinute   = "DEPSCOPE_API_RATE_PER_MINUTE"
	EnvRateBurst       = "DEPSCOPE_API_RATE_BURST"
	EnvCORSOrigins     = "DEPSCOPE_API_CORS_ORIGINS"
	EnvShutdownTimeout = "DEPSCOPE_API_SHUTDOWN_TIMEOUT"
)

const (
	defaultAddr            = "127.0.0.1:8080"
	defaultRequestTimeout  = 90 * time.Second
	defaultMaxBodyBytes    = int64(16 << 10)
	defaultMaxConcurrency  = 8
	defaultRatePerMinute   = 60
	defaultRateBurst       = 10
	defaultShutdownTimeout = 15 * time.Second

	minBodyBytes     = int64(1024)
	maxBodyBytes     = int64(1 << 20)
	maxConcurrency   = 1024
	maxRatePerMinute = 1_000_000
	maxRateBurst     = 100_000
)

// Config 配置 HTTP 服务的进程内资源边界与跨域策略。
// 零值不视为可用配置；请从 DefaultConfig 或 LoadConfigFromEnv 开始。
type Config struct {
	Addr               string
	RequestTimeout     time.Duration
	MaxBodyBytes       int64
	MaxConcurrency     int
	RateLimitPerMinute int
	RateLimitBurst     int
	CORSOrigins        []string
	ShutdownTimeout    time.Duration
}

// DefaultConfig 返回只监听回环地址、默认关闭跨域访问的保守配置。
func DefaultConfig() Config {
	return Config{
		Addr:               defaultAddr,
		RequestTimeout:     defaultRequestTimeout,
		MaxBodyBytes:       defaultMaxBodyBytes,
		MaxConcurrency:     defaultMaxConcurrency,
		RateLimitPerMinute: defaultRatePerMinute,
		RateLimitBurst:     defaultRateBurst,
		CORSOrigins:        nil,
		ShutdownTimeout:    defaultShutdownTimeout,
	}
}

// Validate 检查配置是否处于允许的范围内。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("addr must not be empty")
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return fmt.Errorf("addr %q must use host:port form: %w", c.Addr, err)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if c.MaxBodyBytes < minBodyBytes || c.MaxBodyBytes > maxBodyBytes {
		return fmt.Errorf("max body bytes must be between %d and %d", minBodyBytes, maxBodyBytes)
	}
	if c.MaxConcurrency <= 0 || c.MaxConcurrency > maxConcurrency {
		return fmt.Errorf("max concurrency must be between 1 and %d", maxConcurrency)
	}
	if c.RateLimitPerMinute <= 0 || c.RateLimitPerMinute > maxRatePerMinute {
		return fmt.Errorf("rate limit per minute must be between 1 and %d", maxRatePerMinute)
	}
	if c.RateLimitBurst <= 0 || c.RateLimitBurst > maxRateBurst {
		return fmt.Errorf("rate limit burst must be between 1 and %d", maxRateBurst)
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if _, err := normalizeOrigins(c.CORSOrigins); err != nil {
		return err
	}
	return nil
}

// LoadConfigFromEnv 读取 DEPSCOPE_API_* 环境变量并验证结果。
// getenv 为 nil 时使用 os.Getenv，便于测试注入。
func LoadConfigFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	config := DefaultConfig()

	if value := strings.TrimSpace(getenv(EnvAddr)); value != "" {
		config.Addr = value
	}
	if value := strings.TrimSpace(getenv(EnvRequestTimeout)); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvRequestTimeout, err)
		}
		config.RequestTimeout = parsed
	}
	if value := strings.TrimSpace(getenv(EnvShutdownTimeout)); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvShutdownTimeout, err)
		}
		config.ShutdownTimeout = parsed
	}
	if value := strings.TrimSpace(getenv(EnvMaxBodyBytes)); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvMaxBodyBytes, err)
		}
		config.MaxBodyBytes = parsed
	}
	if value := strings.TrimSpace(getenv(EnvMaxConcurrency)); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvMaxConcurrency, err)
		}
		config.MaxConcurrency = parsed
	}
	if value := strings.TrimSpace(getenv(EnvRatePerMinute)); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvRatePerMinute, err)
		}
		config.RateLimitPerMinute = parsed
	}
	if value := strings.TrimSpace(getenv(EnvRateBurst)); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvRateBurst, err)
		}
		config.RateLimitBurst = parsed
	}
	if value := strings.TrimSpace(getenv(EnvCORSOrigins)); value != "" {
		origins := strings.Split(value, ",")
		normalized, err := normalizeOrigins(origins)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvCORSOrigins, err)
		}
		config.CORSOrigins = normalized
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func normalizeOrigins(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		raw := strings.TrimSpace(value)
		if raw == "" {
			continue
		}
		origin, err := normalizeOrigin(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}
	return result, nil
}

func normalizeOrigin(raw string) (string, error) {
	if raw == "*" {
		return "", fmt.Errorf("wildcard CORS origins are not supported")
	}
	parsed, err := url.Parse(strings.TrimSuffix(raw, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid CORS origin %q", raw)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("CORS origin %q must use http or https", raw)
	}
	if parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("CORS origin %q must be a scheme and host without credentials", raw)
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("CORS origin %q must not include a path, query, or fragment", raw)
	}
	return scheme + "://" + parsed.Host, nil
}
