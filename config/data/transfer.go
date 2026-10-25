package data

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/flare/config/model"
	"gopkg.in/yaml.v2"
)

// All in-process bookmark writers share this lock, including the legacy editor.
var bookmarkMu sync.RWMutex

// ErrBookmarkConflict indicates that a preview no longer matches the on-disk file.
var ErrBookmarkConflict = errors.New("书签已发生变化，请重新预览")

const bookmarkBackupDir = ".flare-bookmark-backups"

func validBookmarkCollection(name string) bool { return name == "bookmarks" || name == "apps" }
func bookmarkVersion(raw []byte) string        { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// BookmarkSnapshot reads disk rather than the display cache to detect external edits.
func BookmarkSnapshot(name string) (model.Bookmarks, string, error) {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	if !validBookmarkCollection(name) {
		return model.Bookmarks{}, "", fmt.Errorf("无效的数据类型")
	}
	path := getConfigPath(name)
	if !checkExists(path) {
		if _, err := initBookmarks(path, name == "apps"); err != nil {
			return model.Bookmarks{}, "", err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Bookmarks{}, "", err
	}
	var src model.Bookmarks
	if err = yaml.UnmarshalStrict(raw, &src); err != nil {
		return src, "", err
	}
	return src, bookmarkVersion(raw), nil
}

// atomicBookmarkWrite stages in the destination directory, syncs and renames on the same filesystem.
func atomicBookmarkWrite(path string, raw []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".flare-transfer-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer func() { _ = os.Remove(temp) }()
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// CommitBookmarkImport backs up and atomically replaces one collection if unchanged.
func CommitBookmarkImport(name, version string, next model.Bookmarks) error {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	if !validBookmarkCollection(name) {
		return fmt.Errorf("无效的数据类型")
	}
	path := getConfigPath(name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bookmarkVersion(raw) != version {
		return ErrBookmarkConflict
	}
	out, err := yaml.Marshal(next)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(bookmarkBackupDir, 0700); err != nil {
		return fmt.Errorf("创建备份目录: %w", err)
	}
	// Reject symlinked backup directories; downloads only use server-generated names.
	info, err := os.Lstat(bookmarkBackupDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("备份目录无效")
	}
	backup := filepath.Join(bookmarkBackupDir, name+"-"+time.Now().UTC().Format("20060102T150405.000000000")+"-"+GenerateRandomString(12)+".yml")
	if err = atomicBookmarkWrite(backup, raw, 0600); err != nil {
		return fmt.Errorf("保存导入前备份: %w", err)
	}
	info, err = os.Stat(path)
	if err != nil {
		return err
	}
	if err = atomicBookmarkWrite(path, out, info.Mode().Perm()); err != nil {
		return err
	}
	invalidateFileCache(name)
	return nil
}

// LatestBookmarkBackup returns the newest server-generated backup for a collection.
func LatestBookmarkBackup(name string) ([]byte, time.Time, error) {
	bookmarkMu.RLock()
	defer bookmarkMu.RUnlock()
	if !validBookmarkCollection(name) {
		return nil, time.Time{}, fmt.Errorf("无效的数据类型")
	}
	entries, err := os.ReadDir(bookmarkBackupDir)
	if err != nil {
		return nil, time.Time{}, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), name+"-") && strings.HasSuffix(entry.Name(), ".yml") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return nil, time.Time{}, os.ErrNotExist
	}
	sort.Strings(names)
	path := filepath.Join(bookmarkBackupDir, names[len(names)-1])
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	raw, err := os.ReadFile(path)
	return raw, info.ModTime(), err
}
