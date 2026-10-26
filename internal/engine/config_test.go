package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

func TestDefaultConfigIsValid(t *testing.T) {
	t.Parallel()

	config := engine.DefaultConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error = %v", err)
	}
	if config.ScanTimeout <= 0 || config.ProviderTimeout <= 0 {
		t.Fatalf("default timeouts must be positive: %+v", config)
	}
	if config.MaxConcurrency <= 0 {
		t.Fatalf("default MaxConcurrency = %d, want positive", config.MaxConcurrency)
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*engine.Config)
		wantErr string
	}{
		{
			name:    "zero scan timeout",
			mutate:  func(c *engine.Config) { c.ScanTimeout = 0 },
			wantErr: "ScanTimeout",
		},
		{
			name:    "zero provider timeout",
			mutate:  func(c *engine.Config) { c.ProviderTimeout = 0 },
			wantErr: "ProviderTimeout",
		},
		{
			name:    "zero concurrency",
			mutate:  func(c *engine.Config) { c.MaxConcurrency = 0 },
			wantErr: "MaxConcurrency",
		},
		{
			name: "cache enabled without ttl",
			mutate: func(c *engine.Config) {
				c.CacheEnabled = true
				c.CacheTTL = 0
			},
			wantErr: "CacheTTL",
		},
		{
			name:    "negative cache ttl",
			mutate:  func(c *engine.Config) { c.CacheTTL = -time.Second },
			wantErr: "CacheTTL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := engine.DefaultConfig()
			tc.mutate(&config)
			err := config.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestConfigCacheDisabledAllowsZeroTTL(t *testing.T) {
	t.Parallel()

	config := engine.DefaultConfig()
	config.CacheEnabled = false
	config.CacheTTL = 0
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigContextsApplyTimeouts(t *testing.T) {
	t.Parallel()

	config := engine.DefaultConfig()
	config.ScanTimeout = time.Nanosecond
	config.ProviderTimeout = time.Nanosecond

	scanCtx, cancelScan := config.ScanContext(context.Background())
	defer cancelScan()
	<-scanCtx.Done()
	if !errors.Is(scanCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("ScanContext error = %v, want deadline exceeded", scanCtx.Err())
	}

	providerCtx, cancelProvider := config.ProviderContext(context.Background())
	defer cancelProvider()
	<-providerCtx.Done()
	if !errors.Is(providerCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("ProviderContext error = %v, want deadline exceeded", providerCtx.Err())
	}
}

func TestConfigContextsRespectParentCancellation(t *testing.T) {
	t.Parallel()

	config := engine.DefaultConfig()
	parent, cancel := context.WithCancel(context.Background())
	ctx, cancelChild := config.ScanContext(parent)
	defer cancelChild()

	cancel()
	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("ScanContext error = %v, want canceled", ctx.Err())
	}
}

func TestLimiterBoundsConcurrencyAndHonorsCancellation(t *testing.T) {
	t.Parallel()

	limiter, err := engine.NewLimiter(2)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelWait()
	if err := limiter.Acquire(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want deadline exceeded", err)
	}

	limiter.Release()
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}

	limiter.Release()
	limiter.Release()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Acquire(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire() error = %v, want canceled", err)
	}
}

func TestNewLimiterRejectsInvalidCapacity(t *testing.T) {
	t.Parallel()

	if _, err := engine.NewLimiter(0); err == nil {
		t.Fatal("NewLimiter(0) error = nil, want error")
	}
}

func TestNoopCache(t *testing.T) {
	t.Parallel()

	var cache engine.Cache = engine.NoopCache{}
	value, found, err := cache.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found || value != nil {
		t.Fatalf("Get() = (%v, %v), want (nil, false)", value, found)
	}
	if err := cache.Set(context.Background(), "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
}

func TestScanRequestValidate(t *testing.T) {
	t.Parallel()

	target := model.ScanTarget{
		Package: model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"},
	}
	valid := engine.ScanRequest{Target: target, DataMode: model.DataModeLive}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalidMode := valid
	invalidMode.DataMode = model.DataMode("cached")
	if err := invalidMode.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want data-mode error")
	}

	invalidTarget := valid
	invalidTarget.Target.Package = model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "Lodash"}
	if err := invalidTarget.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want target error")
	}
}

func TestProviderErrorWrapsCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("test cause")
	providerErr := &engine.ProviderError{
		Provider: "demo",
		Kind:     engine.ProviderErrorUnavailable,
		Err:      cause,
	}
	if !errors.Is(providerErr, cause) {
		t.Fatal("ProviderError does not unwrap to its cause")
	}
	if !strings.Contains(providerErr.Error(), "demo") {
		t.Fatalf("ProviderError.Error() = %q, want provider name", providerErr.Error())
	}
}
