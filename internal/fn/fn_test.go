package fn

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
	out := GetWorkDirFile("any")
	if out == "" {
		t.Fatal("GetWorkDirFile should not return empty")
	}
	if filepath.Base(out) != "any" {
		t.Errorf("GetWorkDirFile should use filename: want base %q, got %q", "any", filepath.Base(out))
	}
	outEnv := GetWorkDirFile(".env")
	if filepath.Base(outEnv) != ".env" {
		t.Errorf("GetWorkDirFile(.env) should end with .env, got %q", outEnv)
	}
}
