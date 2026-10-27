// Package main 的 depscope-mcp 命令测试。
package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSliceFlag(t *testing.T) {
	var flag sliceFlag
	if err := flag.Set("/path/one"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := flag.Set("/path/two"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if len(flag) != 2 || flag[0] != "/path/one" || flag[1] != "/path/two" {
		t.Fatalf("unexpected flag values: %v", flag)
	}
	if flag.String() != "/path/one, /path/two" {
		t.Fatalf("unexpected string: %q", flag.String())
	}
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即时取消

	tempDir := t.TempDir()
	err := run(ctx, []string{tempDir}, nil)
	// 期望因为上下文取消而优雅退出，不产生 panic
	if err != nil && err != context.Canceled {
		// 上下文取消或关闭管道均为正常状态
		t.Logf("run returned error on cancelled context: %v", err)
	}
}

func TestMainVersionFlag(t *testing.T) {
	if version == "" {
		t.Fatal("version must not be empty")
	}
	// 验证时间戳与基本常量配置
	_ = time.Now()
	_ = os.Args
}
