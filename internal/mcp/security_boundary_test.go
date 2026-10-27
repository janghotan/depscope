// Package mcp 的文件系统安全边界深度渗透与攻击模拟测试。
package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathValidatorSecurityBoundaries(t *testing.T) {
	tempBase := t.TempDir()

	// 目录结构：
	// tempBase/
	//   allowed/
	//     project/
	//       package.json
	//     subproject/
	//     link_to_outside -> ../forbidden/
	//     link_to_inside -> project/
	//     nested/
	//       link_out -> ../../forbidden/
	//   forbidden/
	//     secret.txt
	//     project/
	//   link_root -> allowed/

	allowedDir := filepath.Join(tempBase, "allowed")
	forbiddenDir := filepath.Join(tempBase, "forbidden")
	projectDir := filepath.Join(allowedDir, "project")
	subprojectDir := filepath.Join(allowedDir, "subproject")
	nestedDir := filepath.Join(allowedDir, "nested")

	for _, d := range []string{allowedDir, forbiddenDir, projectDir, subprojectDir, nestedDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// 1. 同名目录前缀攻击 (Prefix Collision / Sibling Directory Attack)
	// 例如 allowed="/var/app", attacker requests "/var/app-evil"
	siblingEvilDir := filepath.Join(tempBase, "allowed-evil")
	if err := os.MkdirAll(siblingEvilDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 创建符号链接
	linkToOutside := filepath.Join(allowedDir, "link_to_outside")
	_ = os.Symlink(forbiddenDir, linkToOutside)

	linkToInside := filepath.Join(allowedDir, "link_to_inside")
	_ = os.Symlink(projectDir, linkToInside)

	nestedLinkOut := filepath.Join(nestedDir, "link_out")
	_ = os.Symlink(forbiddenDir, nestedLinkOut)

	linkRoot := filepath.Join(tempBase, "link_root")
	_ = os.Symlink(allowedDir, linkRoot)

	validator, err := NewPathValidator([]string{allowedDir})
	if err != nil {
		t.Fatalf("NewPathValidator: %v", err)
	}

	tests := []struct {
		name      string
		input     string
		wantAllow bool
	}{
		{
			name:      "exact allowed root",
			input:     allowedDir,
			wantAllow: true,
		},
		{
			name:      "valid nested project directory",
			input:     projectDir,
			wantAllow: true,
		},
		{
			name:      "valid trailing slash",
			input:     projectDir + string(filepath.Separator),
			wantAllow: true,
		},
		{
			name:      "sibling directory with common prefix (prefix attack)",
			input:     siblingEvilDir,
			wantAllow: false,
		},
		{
			name:      "relative path traversal with dots (allowed/../forbidden)",
			input:     filepath.Join(allowedDir, "..", "forbidden"),
			wantAllow: false,
		},
		{
			name:      "complex path traversal (allowed/project/../../forbidden)",
			input:     filepath.Join(projectDir, "..", "..", "forbidden"),
			wantAllow: false,
		},
		{
			name:      "absolute path to forbidden directory",
			input:     forbiddenDir,
			wantAllow: false,
		},
		{
			name:      "symlink inside allowed root pointing outside (symlink escape)",
			input:     linkToOutside,
			wantAllow: false,
		},
		{
			name:      "nested symlink inside allowed root pointing outside",
			input:     nestedLinkOut,
			wantAllow: false,
		},
		{
			name:      "symlink inside allowed root pointing to valid subproject",
			input:     linkToInside,
			wantAllow: true,
		},
		{
			name:      "symlinked root path resolved to valid allowed root",
			input:     linkRoot,
			wantAllow: true,
		},
		{
			name:      "root filesystem /",
			input:     "/",
			wantAllow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := validator.ValidatePath(tc.input)
			if tc.wantAllow {
				if err != nil {
					t.Fatalf("expected allowed, got error: %v", err)
				}
				if resolved == "" {
					t.Fatalf("expected non-empty resolved path")
				}
			} else {
				if err == nil {
					t.Fatalf("expected path to be rejected, but got resolved=%q", resolved)
				}
			}
		})
	}
}
