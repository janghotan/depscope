package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	defaultCommandTimeout = 15 * time.Second
	defaultMaxOutput      = 8 << 20
	pipListArgs           = "-m pip list --format=json --disable-pip-version-check"
)

// CommandRunner 执行只读的包管理器查询命令；实现必须遵守 context 取消。
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// System 通过 npm 与 pip 的只读查询命令发现全局安装的包。
// Timeout 与 Runner 为零值时使用受限的默认实现；本类型不会安装、升级或执行任何包。
type System struct {
	Timeout time.Duration
	Runner  CommandRunner
}

// NPMGlobalRoot 通过 "npm root -g" 返回 npm 全局 node_modules 目录。
func (s System) NPMGlobalRoot(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	callCtx, cancel := s.commandContext(ctx)
	defer cancel()

	output, err := s.runner().Run(callCtx, "npm", "root", "-g")
	if err != nil {
		return "", fmt.Errorf("npm root -g failed: %w", err)
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", errors.New("npm root -g returned no path")
	}
	if strings.ContainsAny(root, "\r\n") {
		return "", errors.New("npm root -g returned more than one line")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("npm root -g returned non-absolute path %q", root)
	}
	return filepath.Clean(root), nil
}

// PipPackages 依次尝试 python3 与 python 的 "pip list" 只读输出并解析为包列表。
// 命令失败时才尝试下一个解释器；解析失败会立即返回错误，避免掩盖损坏的输出。
func (s System) PipPackages(ctx context.Context) ([]Package, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	interpreters := []string{"python3", "python"}
	failures := make([]string, 0, len(interpreters))
	for _, interpreter := range interpreters {
		callCtx, cancel := s.commandContext(ctx)
		output, err := s.runner().Run(callCtx, interpreter, "-m", "pip", "list", "--format=json", "--disable-pip-version-check")
		cancel()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			failures = append(failures, fmt.Sprintf("%s: %v", interpreter, err))
			continue
		}
		packages, err := parsePipList(output)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", interpreter, pipListArgs, err)
		}
		return packages, nil
	}
	return nil, fmt.Errorf("pip inventory failed: %s", strings.Join(failures, "; "))
}

func (s System) runner() CommandRunner {
	if s.Runner != nil {
		return s.Runner
	}
	return execRunner{maxOutput: defaultMaxOutput}
}

func (s System) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// parsePipList 严格解析 pip list 的 JSON 数组，拒绝空输出、null 与尾随数据。
func parsePipList(data []byte) ([]Package, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("pip list returned no output")
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	var entries []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse pip list JSON: %w", err)
	}
	if entries == nil {
		return nil, errors.New("pip list returned a null package list")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("pip list returned trailing data")
		}
		return nil, fmt.Errorf("parse pip list JSON: %w", err)
	}

	packages := make([]Package, 0, len(entries))
	for _, entry := range entries {
		packages = append(packages, Package{Name: entry.Name, Version: entry.Version})
	}
	return packages, nil
}

// execRunner 使用不经过 shell 的命令执行，并限制标准输出与错误输出的大小。
type execRunner struct {
	maxOutput int
}

func (r execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	limit := r.maxOutput
	if limit <= 0 {
		limit = defaultMaxOutput
	}

	command := exec.CommandContext(ctx, name, args...)
	stdout := &outputLimiter{limit: limit}
	stderr := &outputLimiter{limit: limit}
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	if stdout.overflow {
		return nil, fmt.Errorf("%s output exceeded %d bytes", name, limit)
	}
	if stderr.overflow {
		return nil, fmt.Errorf("%s error output exceeded %d bytes", name, limit)
	}
	if err != nil {
		detail := sanitizeCommandText(string(stderr.data))
		if detail != "" {
			return nil, fmt.Errorf("%s: %w: %s", name, err, detail)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return stdout.data, nil
}

// outputLimiter 收集命令输出，超过上限时拒绝继续写入并标记溢出。
type outputLimiter struct {
	limit    int
	data     []byte
	overflow bool
}

var errOutputLimit = errors.New("output limit exceeded")

func (w *outputLimiter) Write(p []byte) (int, error) {
	if len(w.data)+len(p) > w.limit {
		w.overflow = true
		return 0, errOutputLimit
	}
	w.data = append(w.data, p...)
	return len(p), nil
}

// sanitizeCommandText 将命令输出压缩为单行，避免控制字符进入报告或终端。
func sanitizeCommandText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}
