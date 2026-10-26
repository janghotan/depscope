package engine

import (
	"context"
	"fmt"
)

// Limiter 通过带缓冲的信号量限制并发提供者调用数量。
type Limiter struct {
	slots chan struct{}
}

// NewLimiter 创建指定容量上限的并发限制器。
func NewLimiter(maxConcurrency int) (*Limiter, error) {
	if maxConcurrency <= 0 {
		return nil, fmt.Errorf("maxConcurrency must be positive")
	}
	return &Limiter{slots: make(chan struct{}, maxConcurrency)}, nil
}

// Acquire 获取一个并发名额；等待期间遵守 context 取消与截止时间。
func (l *Limiter) Acquire(ctx context.Context) error {
	if l == nil {
		return fmt.Errorf("limiter is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release 归还一个并发名额；没有匹配的 Acquire 时会触发 panic。
func (l *Limiter) Release() {
	if l == nil {
		panic("engine: Limiter.Release called on nil limiter")
	}
	select {
	case <-l.slots:
	default:
		panic("engine: Limiter.Release called without a matching Acquire")
	}
}
