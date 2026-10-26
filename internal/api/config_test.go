package api_test

import (
	"testing"
	"time"

	"depscope/internal/api"
)

func TestDefaultConfigIsUsable(t *testing.T) {
	t.Parallel()

	config := api.DefaultConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error = %v", err)
	}
	if config.Addr != "127.0.0.1:8080" {
		t.Fatalf("Addr = %q, want loopback default", config.Addr)
	}
	if config.RequestTimeout != 90*time.Second {
		t.Fatalf("RequestTimeout = %s, want 90s", config.RequestTimeout)
	}
	if config.MaxBodyBytes != 16<<10 {
		t.Fatalf("MaxBodyBytes = %d, want %d", config.MaxBodyBytes, 16<<10)
	}
	if config.MaxConcurrency != 8 {
		t.Fatalf("MaxConcurrency = %d, want 8", config.MaxConcurrency)
	}
	if config.RateLimitPerMinute != 60 {
		t.Fatalf("RateLimitPerMinute = %d, want 60", config.RateLimitPerMinute)
	}
	if config.RateLimitBurst != 10 {
		t.Fatalf("RateLimitBurst = %d, want 10", config.RateLimitBurst)
	}
	if len(config.CORSOrigins) != 0 {
		t.Fatalf("CORSOrigins = %v, want empty", config.CORSOrigins)
	}
	if config.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", config.ShutdownTimeout)
	}
}

func TestLoadConfigFromEnvUsesDefaults(t *testing.T) {
	t.Parallel()

	config, err := api.LoadConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	defaults := api.DefaultConfig()
	if config.Addr != defaults.Addr ||
		config.RequestTimeout != defaults.RequestTimeout ||
		config.MaxBodyBytes != defaults.MaxBodyBytes ||
		config.MaxConcurrency != defaults.MaxConcurrency ||
		config.RateLimitPerMinute != defaults.RateLimitPerMinute ||
		config.RateLimitBurst != defaults.RateLimitBurst ||
		config.ShutdownTimeout != defaults.ShutdownTimeout ||
		len(config.CORSOrigins) != 0 {
		t.Fatalf("LoadConfigFromEnv() = %+v, want defaults", config)
	}
}

func TestLoadConfigFromEnvOverrides(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		api.EnvAddr:            "0.0.0.0:9000",
		api.EnvRequestTimeout:  "45s",
		api.EnvMaxBodyBytes:    "32768",
		api.EnvMaxConcurrency:  "4",
		api.EnvRatePerMinute:   "120",
		api.EnvRateBurst:       "20",
		api.EnvCORSOrigins:     "https://app.example.com, https://admin.example.com/",
		api.EnvShutdownTimeout: "5s",
	}
	config, err := api.LoadConfigFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if config.Addr != "0.0.0.0:9000" {
		t.Fatalf("Addr = %q", config.Addr)
	}
	if config.RequestTimeout != 45*time.Second {
		t.Fatalf("RequestTimeout = %s", config.RequestTimeout)
	}
	if config.MaxBodyBytes != 32768 {
		t.Fatalf("MaxBodyBytes = %d", config.MaxBodyBytes)
	}
	if config.MaxConcurrency != 4 {
		t.Fatalf("MaxConcurrency = %d", config.MaxConcurrency)
	}
	if config.RateLimitPerMinute != 120 {
		t.Fatalf("RateLimitPerMinute = %d", config.RateLimitPerMinute)
	}
	if config.RateLimitBurst != 20 {
		t.Fatalf("RateLimitBurst = %d", config.RateLimitBurst)
	}
	if len(config.CORSOrigins) != 2 || config.CORSOrigins[0] != "https://app.example.com" || config.CORSOrigins[1] != "https://admin.example.com" {
		t.Fatalf("CORSOrigins = %v", config.CORSOrigins)
	}
	if config.ShutdownTimeout != 5*time.Second {
		t.Fatalf("ShutdownTimeout = %s", config.ShutdownTimeout)
	}
}

func TestLoadConfigFromEnvRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "request timeout", key: api.EnvRequestTimeout, value: "soon"},
		{name: "zero request timeout", key: api.EnvRequestTimeout, value: "0s"},
		{name: "body bytes", key: api.EnvMaxBodyBytes, value: "large"},
		{name: "small body limit", key: api.EnvMaxBodyBytes, value: "16"},
		{name: "concurrency", key: api.EnvMaxConcurrency, value: "0"},
		{name: "rate", key: api.EnvRatePerMinute, value: "0"},
		{name: "burst", key: api.EnvRateBurst, value: "0"},
		{name: "shutdown", key: api.EnvShutdownTimeout, value: "0s"},
		{name: "wildcard origin", key: api.EnvCORSOrigins, value: "*"},
		{name: "origin with path", key: api.EnvCORSOrigins, value: "https://app.example.com/api"},
		{name: "origin with credentials", key: api.EnvCORSOrigins, value: "https://user@app.example.com"},
		{name: "unsupported origin scheme", key: api.EnvCORSOrigins, value: "ftp://app.example.com"},
		{name: "addr without port", key: api.EnvAddr, value: "127.0.0.1"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := api.LoadConfigFromEnv(func(key string) string {
				if key == test.key {
					return test.value
				}
				return ""
			})
			if err == nil {
				t.Fatalf("LoadConfigFromEnv() error = nil, want error for %s=%q", test.key, test.value)
			}
		})
	}
}

func TestConfigValidateRejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*api.Config)
	}{
		{name: "empty addr", modify: func(c *api.Config) { c.Addr = "" }},
		{name: "zero timeout", modify: func(c *api.Config) { c.RequestTimeout = 0 }},
		{name: "zero body limit", modify: func(c *api.Config) { c.MaxBodyBytes = 0 }},
		{name: "body limit too large", modify: func(c *api.Config) { c.MaxBodyBytes = 2 << 20 }},
		{name: "zero concurrency", modify: func(c *api.Config) { c.MaxConcurrency = 0 }},
		{name: "zero rate", modify: func(c *api.Config) { c.RateLimitPerMinute = 0 }},
		{name: "zero burst", modify: func(c *api.Config) { c.RateLimitBurst = 0 }},
		{name: "zero shutdown", modify: func(c *api.Config) { c.ShutdownTimeout = 0 }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := api.DefaultConfig()
			test.modify(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}
