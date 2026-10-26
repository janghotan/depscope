package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/cache"
)

func TestMemoryCacheExpiresEntries(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cache.NewMemoryCache(8, func() time.Time { return now })
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := c.Get(ctx, "k")
	if err != nil || !found || string(value) != "v" {
		t.Fatalf("Get before expiry = %q, %v, %v", value, found, err)
	}

	now = now.Add(time.Minute + time.Second)
	value, found, err = c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if found || value != nil {
		t.Fatalf("Get after expiry = %q, %v, want miss", value, found)
	}
}

func TestMemoryCacheIsBounded(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cache.NewMemoryCache(2, func() time.Time { return now })
	ctx := context.Background()

	for _, key := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, key, []byte(key), time.Minute); err != nil {
			t.Fatalf("Set %s: %v", key, err)
		}
	}
	if got := c.Len(); got > 2 {
		t.Fatalf("Len = %d, want <= 2", got)
	}
}

func TestMemoryCacheCopiesValues(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cache.NewMemoryCache(2, func() time.Time { return now })
	ctx := context.Background()
	value := []byte("original")

	if err := c.Set(ctx, "k", value, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value[0] = 'X'
	got, _, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("cached value = %q, want original", got)
	}
	got[0] = 'Y'
	again, _, _ := c.Get(ctx, "k")
	if string(again) != "original" {
		t.Fatalf("second read = %q, want original", again)
	}
}

func TestMemoryCacheIgnoresNonPositiveTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cache.NewMemoryCache(2, func() time.Time { return now })

	if err := c.Set(context.Background(), "k", []byte("v"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	_, found, err := c.Get(context.Background(), "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("entry with zero TTL should not be stored")
	}
}
