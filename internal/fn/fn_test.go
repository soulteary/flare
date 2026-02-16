package FlareFn

import (
	"path/filepath"
	"testing"
)

func TestGetWorkDir(t *testing.T) {
	dir := GetWorkDir()
	if dir == "" {
		t.Fatal("GetWorkDir should not return empty string")
	}
}

func TestGetWorkDirFile(t *testing.T) {
	// GetWorkDirFile 当前实现固定使用 ".env"，与参数无关，这里只测返回路径格式
	out := GetWorkDirFile("any")
	if out == "" {
		t.Fatal("GetWorkDirFile should not return empty")
	}
	if filepath.Base(out) != ".env" {
		t.Errorf("GetWorkDirFile should end with .env, got %q", out)
	}
}
