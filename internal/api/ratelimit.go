package api

import (
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	bucketSweepInterval = time.Minute
	bucketIdleTTL       = 10 * time.Minute
	maxTrackedClients   = 20_000
)

type rateBucket struct {
	tokens float64
	last   time.Time
}

// rateLimiter 是进程内令牌桶，按客户端地址限流；多实例部署时每个进程独立计数。
type rateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*rateBucket
	capacity        float64
	refillPerSecond float64
	now             func() time.Time
	lastSweep       time.Time
}

func newRateLimiter(perMinute, burst int, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		buckets:         make(map[string]*rateBucket),
		capacity:        float64(burst),
		refillPerSecond: float64(perMinute) / 60,
		now:             now,
		lastSweep:       now(),
	}
}

// allow 消耗一个令牌；被拒绝时返回需要等待的时长。
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &rateBucket{tokens: l.capacity, last: now}
		l.buckets[key] = bucket
	}
	if elapsed := now.Sub(bucket.last).Seconds(); elapsed > 0 {
		bucket.tokens = math.Min(l.capacity, bucket.tokens+elapsed*l.refillPerSecond)
		bucket.last = now
	}
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	wait := (1 - bucket.tokens) / l.refillPerSecond
	return false, time.Duration(wait * float64(time.Second))
}

// sweep 定期清理空闲令牌桶，并在极端数量下重置映射以避免无界内存增长。
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < bucketSweepInterval {
		return
	}
	l.lastSweep = now
	cutoff := now.Add(-bucketIdleTTL)
	for key, bucket := range l.buckets {
		if bucket.last.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
	if len(l.buckets) > maxTrackedClients {
		l.buckets = make(map[string]*rateBucket)
	}
}

// clientIP 只使用 TCP 对端地址；不信任可伪造的转发头。
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
