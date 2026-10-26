package engine

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultScanTimeout 是单次扫描的默认总超时。
	DefaultScanTimeout = 2 * time.Minute
	// DefaultProviderTimeout 是单次外部提供者调用的默认超时。
	DefaultProviderTimeout = 15 * time.Second
	// DefaultMaxConcurrency 是默认的并发上限。
	DefaultMaxConcurrency = 8
	// DefaultCacheTTL 是启用缓存时的默认生存时间。
	DefaultCacheTTL = 10 * time.Minute
)

// Config 定义扫描编排的可配置运行时限制。
type Config struct {
	ScanTimeout     time.Duration
	ProviderTimeout time.Duration
	MaxConcurrency  int
	CacheEnabled    bool
	CacheTTL        time.Duration
}

// DefaultConfig 返回保守的默认扫描配置。
func DefaultConfig() Config {
	return Config{
		ScanTimeout:     DefaultScanTimeout,
		ProviderTimeout: DefaultProviderTimeout,
		MaxConcurrency:  DefaultMaxConcurrency,
		CacheEnabled:    true,
		CacheTTL:        DefaultCacheTTL,
	}
}

// Validate 检查配置值是否可用。
func (c Config) Validate() error {
	if c.ScanTimeout <= 0 {
		return fmt.Errorf("ScanTimeout must be positive")
	}
	if c.ProviderTimeout <= 0 {
		return fmt.Errorf("ProviderTimeout must be positive")
	}
	if c.MaxConcurrency <= 0 {
		return fmt.Errorf("MaxConcurrency must be positive")
	}
	if c.CacheTTL < 0 {
		return fmt.Errorf("CacheTTL must not be negative")
	}
	if c.CacheEnabled && c.CacheTTL == 0 {
		return fmt.Errorf("CacheTTL must be positive when caching is enabled")
	}
	return nil
}

// ScanContext 在父 context 上应用扫描总超时。
func (c Config) ScanContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, c.ScanTimeout)
}

// ProviderContext 在父 context 上应用单次提供者调用超时。
func (c Config) ProviderContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, c.ProviderTimeout)
}
