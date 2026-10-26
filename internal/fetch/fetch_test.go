package fetch_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"depscope/internal/cache"
	"depscope/internal/engine"
	"depscope/internal/fetch"
)

type payload struct {
	Value string `json:"value"`
}

func newClient(t *testing.T, now *time.Time) *fetch.Client {
	t.Helper()
	return fetch.New(fetch.Config{
		Cache:        cache.NewMemoryCache(8, func() time.Time { return *now }),
		CacheEnabled: true,
		CacheTTL:     time.Minute,
		MaxRetries:   1,
		RetryBackoff: 0,
		Now:          func() time.Time { return *now },
	})
}

func TestGetJSONCachesSuccessfulResponses(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent must be set")
		}
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	client := newClient(t, &now)
	for i := 0; i < 2; i++ {
		var out payload
		if err := client.GetJSON(context.Background(), "test", server.URL, &out); err != nil {
			t.Fatalf("GetJSON: %v", err)
		}
		if out.Value != "ok" {
			t.Fatalf("Value = %q", out.Value)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want 1", got)
	}
}

func TestGetJSONCachePreservesOriginalRetrievedAt(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	client := newClient(t, &now)
	var first payload
	firstRetrievedAt, err := client.GetJSONWithRetrievedAt(context.Background(), "test", server.URL, &first)
	if err != nil {
		t.Fatalf("first GetJSONWithRetrievedAt: %v", err)
	}

	now = now.Add(30 * time.Second)
	var cached payload
	cachedRetrievedAt, err := client.GetJSONWithRetrievedAt(context.Background(), "test", server.URL, &cached)
	if err != nil {
		t.Fatalf("cached GetJSONWithRetrievedAt: %v", err)
	}
	if !cachedRetrievedAt.Equal(firstRetrievedAt) {
		t.Fatalf("cached RetrievedAt = %v, want original fetch time %v", cachedRetrievedAt, firstRetrievedAt)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want 1 cache hit", got)
	}
}

func TestGetJSONRefetchesAfterExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	client := newClient(t, &now)
	var out payload
	if err := client.GetJSON(context.Background(), "test", server.URL, &out); err != nil {
		t.Fatalf("first GetJSON: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if err := client.GetJSON(context.Background(), "test", server.URL, &out); err != nil {
		t.Fatalf("second GetJSON: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("server calls = %d, want 2", got)
	}
}

func TestGetJSONClassifiesHTTPFailures(t *testing.T) {
	now := time.Now()
	tests := []struct {
		status int
		want   engine.ProviderErrorKind
	}{
		{http.StatusNotFound, engine.ProviderErrorNotFound},
		{http.StatusTooManyRequests, engine.ProviderErrorRateLimited},
		{http.StatusBadGateway, engine.ProviderErrorUnavailable},
		{http.StatusBadRequest, engine.ProviderErrorInvalidResponse},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{"error":"nope"}`))
			}))
			defer server.Close()

			client := newClient(t, &now)
			var out payload
			err := client.GetJSON(context.Background(), "test", server.URL, &out)
			var providerErr *engine.ProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error = %v, want ProviderError", err)
			}
			if providerErr.Kind != test.want {
				t.Fatalf("kind = %q, want %q", providerErr.Kind, test.want)
			}
		})
	}
}

func TestGetJSONRejectsMalformedJSON(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":`))
	}))
	defer server.Close()

	client := newClient(t, &now)
	var out payload
	err := client.GetJSON(context.Background(), "test", server.URL, &out)
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorInvalidResponse {
		t.Fatalf("error = %v, want invalid_response", err)
	}
}

func TestGetJSONEnforcesBodyLimit(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 256)))
	}))
	defer server.Close()

	client := fetch.New(fetch.Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute,
		MaxBodyBytes: 64,
		Now:          func() time.Time { return now },
	})
	var out payload
	err := client.GetJSON(context.Background(), "test", server.URL, &out)
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorInvalidResponse {
		t.Fatalf("error = %v, want invalid_response", err)
	}
}

func TestGetJSONRetriesTransientFailure(t *testing.T) {
	now := time.Now()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	}))
	defer server.Close()

	client := newClient(t, &now)
	var out payload
	if err := client.GetJSON(context.Background(), "test", server.URL, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("server calls = %d, want 2", got)
	}
}

func TestGetJSONHonorsCancellation(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := newClient(t, &now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out payload
	err := client.GetJSON(ctx, "test", server.URL, &out)
	if err == nil {
		t.Fatal("expected an error")
	}
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %v, want ProviderError", err)
	}
}
