// Package live 将真实 npm、PyPI 与 OSV 提供者装配为共享安全引擎扫描器。
package live

import (
	"fmt"
	"net/http"
	"time"

	"github.com/janghotan/depscope/internal/cache"
	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/fetch"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/sources/npm"
	"github.com/janghotan/depscope/internal/sources/osv"
	"github.com/janghotan/depscope/internal/sources/pypi"
)

const (
	defaultCacheEntries = 512
	defaultMaxRetries   = 2
	defaultRetryBackoff = 250 * time.Millisecond
)

// Options 配置真实来源扫描器；零值 Config 使用引擎默认配置。
type Options struct {
	Config      engine.Config
	HTTPClient  *http.Client
	NPMBaseURL  string
	PyPIBaseURL string
	OSVBaseURL  string
	Cache       engine.Cache
	Typosquat   *engine.TyposquatChecker
	MaxRetries  int
	Clock       func() time.Time
}

// NewScanner 创建共享 HTTP 客户端、缓存与三个真实来源提供者。
// 默认允许有限次数的瞬时错误重试；所有调用仍受扫描与提供者超时约束。
func NewScanner(opts Options) (*engine.Scanner, error) {
	if opts.Config == (engine.Config{}) {
		opts.Config = engine.DefaultConfig()
	}
	if err := opts.Config.Validate(); err != nil {
		return nil, fmt.Errorf("live scanner config: %w", err)
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}

	store := opts.Cache
	if store == nil {
		if opts.Config.CacheEnabled {
			store = cache.NewMemoryCache(defaultCacheEntries, clock)
		} else {
			store = engine.NoopCache{}
		}
	}
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	client := fetch.New(fetch.Config{
		HTTPClient:   opts.HTTPClient,
		Cache:        store,
		CacheEnabled: opts.Config.CacheEnabled,
		CacheTTL:     opts.Config.CacheTTL,
		MaxRetries:   maxRetries,
		RetryBackoff: defaultRetryBackoff,
		Now:          clock,
	})

	npmProvider := npm.NewProvider(npm.Options{BaseURL: opts.NPMBaseURL, Client: client, Now: clock})
	pypiProvider := pypi.NewProvider(pypi.Options{BaseURL: opts.PyPIBaseURL, Client: client, Now: clock})
	osvProvider := osv.NewProvider(osv.Options{BaseURL: opts.OSVBaseURL, Client: client, Now: clock})

	return engine.NewScanner(engine.ScannerOptions{
		Config: opts.Config,
		Registries: map[model.Ecosystem]engine.RegistryMetadataProvider{
			model.EcosystemNPM:  npmProvider,
			model.EcosystemPyPI: pypiProvider,
		},
		Vulnerabilities: osvProvider,
		Risk:            engine.RulesRiskEvaluator{},
		Typosquat:       opts.Typosquat,
		Clock:           clock,
	})
}
