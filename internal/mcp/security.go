// Package mcp 提供基于官方 Go MCP SDK 的 Model Context Protocol 本地服务端集成。
// 仅允许经由现有安全引擎与项目扫描器执行只读检测，绝不执行包代码或外部命令。
package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathValidator 负责限制与校验项目扫描的文件系统路径，防止目录遍历与未授权文件访问。
type PathValidator struct {
	allowedRoots []string
}

// NewPathValidator 根据允许的基础目录列表创建路径校验器。
// 若未指定任何允许目录，则默认仅允许当前工作目录。
func NewPathValidator(allowedRoots []string) (*PathValidator, error) {
	if len(allowedRoots) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		allowedRoots = []string{wd}
	}

	canonicalRoots := make([]string, 0, len(allowedRoots))
	for _, root := range allowedRoots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed root %q: %w", root, err)
		}
		realRoot, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed root symlinks for %q: %w", root, err)
		}
		info, err := os.Stat(realRoot)
		if err != nil {
			return nil, fmt.Errorf("stat allowed root %q: %w", root, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("allowed root %q is not a directory", root)
		}
		canonicalRoots = append(canonicalRoots, filepath.Clean(realRoot))
	}

	if len(canonicalRoots) == 0 {
		return nil, fmt.Errorf("at least one valid allowed path must be provided")
	}

	return &PathValidator{
		allowedRoots: canonicalRoots,
	}, nil
}

// AllowedRoots 返回已规范化的允许根目录列表。
func (v *PathValidator) AllowedRoots() []string {
	result := make([]string, len(v.allowedRoots))
	copy(result, v.allowedRoots)
	return result
}

// ValidatePath 校验目标路径是否合法、存在、为目录且完全处于受信任的根目录之内。
func (v *PathValidator) ValidatePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", fmt.Errorf("project path must not be empty")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("project path %q does not exist", trimmed)
		}
		return "", fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path %q is not a directory", trimmed)
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("evaluate symlinks for %q: %w", trimmed, err)
	}
	realPath = filepath.Clean(realPath)

	for _, root := range v.allowedRoots {
		rel, err := filepath.Rel(root, realPath)
		if err != nil {
			continue
		}
		if rel == "." {
			return realPath, nil
		}
		if !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel) {
			return realPath, nil
		}
	}

	return "", fmt.Errorf("access denied: path %q is outside permitted project directories", trimmed)
}
