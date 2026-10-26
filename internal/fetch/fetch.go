// Package fetch 提供带缓存、超时与有限重试的 JSON HTTP 客户端。
package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"depscope/internal/engine"
)

const (
	defaultUserAgent    = "DepScope/0.1"
	defaultMaxBodyBytes = 8 << 20
	maxRetryAfter       = 2 * time.Second
)

// Config 定义共享 HTTP 客户端的行为。
type Config struct {
	HTTPClient   *http.Client
	Cache        engine.Cache
	CacheEnabled bool
	CacheTTL     time.Duration
	UserAgent    string
	MaxBodyBytes int64
	MaxRetries   int
	RetryBackoff time.Duration
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
}

// Client 负责获取并解码外部 JSON 响应。
type Client struct {
	httpClient   *http.Client
	cache        engine.Cache
	cacheEnabled bool
	cacheTTL     time.Duration
	userAgent    string
	maxBodyBytes int64
	maxRetries   int
	retryBackoff time.Duration
	now          func() time.Time
	sleep        func(context.Context, time.Duration) error
}

// New 创建客户端；未提供的配置使用保守默认值。
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.Cache == nil {
		cfg.Cache = engine.NoopCache{}
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = sleepContext
	}
	return &Client{
		httpClient:   cfg.HTTPClient,
		cache:        cfg.Cache,
		cacheEnabled: cfg.CacheEnabled && cfg.CacheTTL > 0,
		cacheTTL:     cfg.CacheTTL,
		userAgent:    cfg.UserAgent,
		maxBodyBytes: cfg.MaxBodyBytes,
		maxRetries:   cfg.MaxRetries,
		retryBackoff: cfg.RetryBackoff,
		now:          cfg.Now,
		sleep:        cfg.Sleep,
	}
}

// GetJSON 发起 GET 请求并解码 JSON。
func (c *Client) GetJSON(ctx context.Context, provider, url string, out any) error {
	return c.doJSON(ctx, provider, http.MethodGet, url, nil, out)
}

// PostJSON 发起 POST 请求并解码 JSON 响应。
func (c *Client) PostJSON(ctx context.Context, provider, url string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return &engine.ProviderError{Provider: provider, Kind: engine.ProviderErrorInvalidResponse, Err: fmt.Errorf("encode request: %w", err)}
	}
	return c.doJSON(ctx, provider, http.MethodPost, url, data, out)
}

func (c *Client) doJSON(ctx context.Context, provider, method, url string, body []byte, out any) error {
	if err := ctx.Err(); err != nil {
		return classifyContextError(provider, err)
	}
	cacheKey := c.cacheKey(method, url, body)
	if c.cacheEnabled {
		if data, found, err := c.cache.Get(ctx, cacheKey); err == nil && found {
			if err := json.Unmarshal(data, out); err == nil {
				return nil
			}
		} else if err != nil {
			return classifyContextError(provider, err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		data, status, retryAfter, err := c.send(ctx, method, url, body)
		if err != nil {
			return classifyTransportError(provider, err)
		}
		if status == http.StatusOK {
			if int64(len(data)) > c.maxBodyBytes {
				return &engine.ProviderError{
					Provider: provider,
					Kind:     engine.ProviderErrorInvalidResponse,
					Err:      fmt.Errorf("response body exceeds %d bytes", c.maxBodyBytes),
				}
			}
			if err := json.Unmarshal(data, out); err != nil {
				return &engine.ProviderError{
					Provider: provider,
					Kind:     engine.ProviderErrorInvalidResponse,
					Err:      fmt.Errorf("decode JSON: %w", err),
				}
			}
			if c.cacheEnabled {
				_ = c.cache.Set(ctx, cacheKey, data, c.cacheTTL)
			}
			return nil
		}

		providerErr := statusError(provider, status)
		lastErr = providerErr
		if !retryableStatus(status) || attempt == c.maxRetries {
			return providerErr
		}
		delay := c.retryBackoff
		if retryAfter > delay {
			delay = retryAfter
		}
		if delay > maxRetryAfter {
			delay = maxRetryAfter
		}
		if delay > 0 {
			if err := c.sleep(ctx, delay); err != nil {
				return classifyContextError(provider, err)
			}
		}
	}
	return lastErr
}

func (c *Client) send(ctx context.Context, method, url string, body []byte) ([]byte, int, time.Duration, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, 0, 0, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, c.maxBodyBytes+1))
	if err != nil {
		return nil, response.StatusCode, 0, err
	}
	return data, response.StatusCode, parseRetryAfter(response.Header.Get("Retry-After")), nil
}

func (c *Client) cacheKey(method, url string, body []byte) string {
	digest := sha256.Sum256(body)
	return "depscope:fetch:v1:" + method + ":" + url + ":" + hex.EncodeToString(digest[:8])
}

func statusError(provider string, status int) *engine.ProviderError {
	var kind engine.ProviderErrorKind
	switch {
	case status == http.StatusNotFound:
		kind = engine.ProviderErrorNotFound
	case status == http.StatusTooManyRequests:
		kind = engine.ProviderErrorRateLimited
	case status >= 500:
		kind = engine.ProviderErrorUnavailable
	case status >= 400:
		kind = engine.ProviderErrorInvalidResponse
	default:
		kind = engine.ProviderErrorInvalidResponse
	}
	return &engine.ProviderError{
		Provider: provider,
		Kind:     kind,
		Err:      fmt.Errorf("unexpected HTTP status %d", status),
	}
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func classifyTransportError(provider string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &engine.ProviderError{Provider: provider, Kind: engine.ProviderErrorTimeout, Err: err}
	}
	return &engine.ProviderError{Provider: provider, Kind: engine.ProviderErrorUnavailable, Err: err}
}

func classifyContextError(provider string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &engine.ProviderError{Provider: provider, Kind: engine.ProviderErrorTimeout, Err: err}
	}
	return &engine.ProviderError{Provider: provider, Kind: engine.ProviderErrorUnavailable, Err: err}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
