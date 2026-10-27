// Package mcp 的安全路径校验与目录限制单元测试。
package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathValidatorAllowedRoots(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "allowed-project")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("mkdir subDir: %v", err)
	}

	validator, err := NewPathValidator([]string{subDir})
	if err != nil {
		t.Fatalf("NewPathValidator: %v", err)
	}

	// 1. 允许目录自身
	valid, err := validator.ValidatePath(subDir)
	if err != nil {
		t.Fatalf("ValidatePath(subDir) failed: %v", err)
	}
	if valid != subDir {
		t.Fatalf("valid = %q, want %q", valid, subDir)
	}

	// 2. 允许目录下的子目录
	childDir := filepath.Join(subDir, "packages", "pkg-a")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("mkdirAll childDir: %v", err)
	}
	validChild, err := validator.ValidatePath(childDir)
	if err != nil {
		t.Fatalf("ValidatePath(childDir) failed: %v", err)
	}
	if validChild != childDir {
		t.Fatalf("validChild = %q, want %q", validChild, childDir)
	}

	// 3. 允许目录之外的目录应拒绝
	outsideDir := filepath.Join(tempDir, "forbidden")
	if err := os.Mkdir(outsideDir, 0755); err != nil {
		t.Fatalf("mkdir outsideDir: %v", err)
	}
	_, err = validator.ValidatePath(outsideDir)
	if err == nil {
		t.Fatal("expected error for path outside permitted root, got nil")
	}

	// 4. 不存在的目录应报错
	_, err = validator.ValidatePath(filepath.Join(subDir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}

	// 5. 文件（而非目录）应报错
	filePath := filepath.Join(subDir, "package.json")
	if err := os.WriteFile(filePath, []byte("{}"), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	_, err = validator.ValidatePath(filePath)
	if err == nil {
		t.Fatal("expected error when validating a regular file, got nil")
	}

	// 6. 空路径校验
	_, err = validator.ValidatePath("")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestPathValidatorDefaultWorkingDir(t *testing.T) {
	validator, err := NewPathValidator(nil)
	if err != nil {
		t.Fatalf("NewPathValidator(nil): %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	allowed := validator.AllowedRoots()
	if len(allowed) != 1 || allowed[0] != wd {
		t.Fatalf("allowed = %v, want [%s]", allowed, wd)
	}

	// 当前工作目录必须通过校验
	valid, err := validator.ValidatePath(wd)
	if err != nil {
		t.Fatalf("validate working directory: %v", err)
	}
	if valid != wd {
		t.Fatalf("valid = %q, want %q", valid, wd)
	}
}
