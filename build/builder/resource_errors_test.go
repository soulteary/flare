package builder

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestResourceTasksCopyAssets(t *testing.T) {
	for _, task := range []struct {
		name string
		run  func(string, string)
	}{{"editor", TaskForEditorAssets}, {"guide", TaskForGuideAssets}} {
		t.Run(task.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			dest := filepath.Join(dir, "dest")
			require.NoError(t, os.Mkdir(src, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(src, "app.js"), []byte("asset sentinel"), 0600))
			task.run(src, dest)
			raw, err := os.ReadFile(filepath.Join(dest, "app.js"))
			require.NoError(t, err)
			require.Equal(t, "asset sentinel", string(raw))
		})
	}
	dest := filepath.Join(t.TempDir(), "simple")
	TaskForSimpleIcons(dest)
	info, err := os.Stat(dest)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestCopyReportsInvalidDestinations(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0700))
	block := filepath.Join(dir, "block")
	require.NoError(t, os.WriteFile(block, []byte("keep"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "asset"), []byte("nested"), 0600))
	require.Error(t, _CopyDirectoryWithoutSymlink(src, block))
	require.NoError(t, os.WriteFile(filepath.Join(src, "asset"), []byte("data"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dest"), 0700))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dest", "asset"), 0700))
	require.Error(t, _CopyDirectoryWithoutSymlink(src, filepath.Join(dir, "dest")))
	require.Error(t, _Copy(src, filepath.Join(dir, "copy")))
	raw, err := os.ReadFile(block)
	require.NoError(t, err)
	require.Equal(t, "keep", string(raw))
}

func TestFaviconRejectsCorruptInput(t *testing.T) {
	raw, err := os.ReadFile("../../embed/assets/favicon.ico")
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"short header", func(b []byte) []byte { return b[:3] }},
		{"wrong type", func(b []byte) []byte { b[2] = 2; return b }},
		{"truncated directory", func(b []byte) []byte { return b[:10] }},
		{"unsupported sizes", func(b []byte) []byte { b[6] = 64; return b }},
		{"image out of bounds", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[18:22], uint32(len(b))); return b }},
		{"invalid png", func(b []byte) []byte { start := binary.LittleEndian.Uint32(b[18:22]); b[start] = 0; return b }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := colorFavicon(tc.mutate(append([]byte(nil), raw...)), "#123456")
			require.Error(t, err)
		})
	}
	_, err = colorFavicon(raw, "invalid")
	require.Error(t, err)
	_, err = darkestPaletteColor(model.Palette{Background: "invalid"})
	require.Error(t, err)
}

func TestFaviconBuildFilesystemFailures(t *testing.T) {
	source := "../../embed/assets/favicon.ico"
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string) (string, string)
	}{
		{"missing source directory", func(t *testing.T, dir string) (string, string) {
			return filepath.Join(dir, "missing", "icon.ico"), filepath.Join(dir, "icon.ico")
		}},
		{"missing source", func(t *testing.T, dir string) (string, string) {
			return filepath.Join(dir, "missing.ico"), filepath.Join(dir, "icon.ico")
		}},
		{"missing destination directory", func(t *testing.T, dir string) (string, string) {
			return source, filepath.Join(dir, "missing", "icon.ico")
		}},
		{"destination is directory", func(t *testing.T, dir string) (string, string) {
			dest := filepath.Join(dir, "icon.ico")
			require.NoError(t, os.Mkdir(dest, 0700))
			return source, dest
		}},
		{"favicon directory is file", func(t *testing.T, dir string) (string, string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "favicons"), []byte("keep"), 0600))
			return source, filepath.Join(dir, "icon.ico")
		}},
		{"corrupt source", func(t *testing.T, dir string) (string, string) {
			src := filepath.Join(dir, "source.ico")
			require.NoError(t, os.WriteFile(src, []byte("corrupt"), 0600))
			return src, filepath.Join(dir, "icon.ico")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { src, dest := tc.setup(t, t.TempDir()); require.Error(t, buildFavicons(src, dest)) })
	}
	dest := filepath.Join(t.TempDir(), "favicon.ico")
	TaskForFavicon(source, dest)
	raw, err := os.ReadFile(dest)
	require.NoError(t, err)
	original, err := os.ReadFile(source)
	require.NoError(t, err)
	require.Equal(t, original, raw)
}
