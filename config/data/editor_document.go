package data

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/flare/config/model"
	"gopkg.in/yaml.v2"
)

// EditorDocument retains category IDs and private flags across every editor view.
// Versions cover the on-disk bytes of BOTH collections, including external edits.
type EditorDocument struct {
	Version    string           `json:"version"`
	Categories []model.Category `json:"categories"`
	Apps       []model.Bookmark `json:"apps"`
	Bookmarks  []model.Bookmark `json:"bookmarks"`
}

type EditorValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *EditorValidationError) Error() string { return e.Message }

func editorVersion(apps, bookmarks []byte) string {
	raw := append([]byte(fmt.Sprintf("%d:", len(apps))), apps...)
	raw = append(raw, bookmarks...)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func editorSnapshotLocked() (EditorDocument, [2][]byte, error) {
	var doc EditorDocument
	var raw [2][]byte
	for i, name := range []string{"apps", "bookmarks"} {
		file := getConfigPath(name)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if _, err = loadBookmarksFromYamlFile(name, i == 0); err != nil {
				return doc, raw, err
			}
		}
		b, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return doc, raw, err
		}
		raw[i] = b
	}
	var apps, bookmarks model.Bookmarks
	if err := yaml.UnmarshalStrict(raw[0], &apps); err != nil {
		return doc, raw, err
	}
	if err := yaml.UnmarshalStrict(raw[1], &bookmarks); err != nil {
		return doc, raw, err
	}
	doc = EditorDocument{Version: editorVersion(raw[0], raw[1]), Categories: bookmarks.Categories, Apps: apps.Items, Bookmarks: bookmarks.Items}
	return doc, raw, nil
}

func EditorSnapshot() (EditorDocument, error) {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	doc, _, err := editorSnapshotLocked()
	return doc, err
}

func ValidateEditorDocument(doc EditorDocument) error {
	invalid := func(field, message string) error { return &EditorValidationError{field, message} }
	ids, names := map[string]bool{}, map[string]bool{}
	for i, c := range doc.Categories {
		field := fmt.Sprintf("categories.%d", i)
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] {
			return invalid(field+".ID", "分类 ID 缺失或重复")
		}
		name := strings.TrimSpace(c.Name)
		if name == "" || name == "[Flare 应用]" || names[name] {
			return invalid(field+".Name", "分类名称不能为空、重复或使用保留名称")
		}
		ids[c.ID], names[name] = true, true
	}
	for _, group := range []struct {
		name  string
		items []model.Bookmark
	}{{"apps", doc.Apps}, {"bookmarks", doc.Bookmarks}} {
		for i, b := range group.items {
			field := fmt.Sprintf("%s.%d", group.name, i)
			if strings.TrimSpace(b.Name) == "" {
				return invalid(field+".Name", "请填写名称")
			}
			if strings.TrimSpace(b.URL) == "" {
				return invalid(field+".URL", "请填写地址")
			}
			// Preserve existing custom schemes, intranet URLs and their exact encoding.
			if group.name == "apps" && b.Category != "" {
				return invalid(field+".Category", "应用不能引用书签分类")
			}
			if group.name == "bookmarks" && b.Category != "" && !ids[b.Category] {
				return invalid(field+".Category", "书签引用的分类不存在，请选择有效位置")
			}
		}
	}
	return nil
}

// CommitEditorDocument prepares both files and recovery copies before replacing
// either destination. This is not a cross-file filesystem transaction. A process
// crash can leave a partial replacement; recovery copies are retained on failure.
func CommitEditorDocument(next EditorDocument) (EditorDocument, error) {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	return commitEditorDocument(next, os.Rename)
}
func commitEditorDocument(next EditorDocument, rename func(string, string) error) (EditorDocument, error) {
	if err := ValidateEditorDocument(next); err != nil {
		return EditorDocument{}, err
	}
	before, raw, err := editorSnapshotLocked()
	if err != nil {
		return EditorDocument{}, err
	}
	if next.Version == "" || next.Version != before.Version {
		return EditorDocument{}, ErrBookmarkConflict
	}
	output := [2][]byte{}
	output[0], err = yaml.Marshal(model.Bookmarks{Items: next.Apps})
	if err != nil {
		return EditorDocument{}, err
	}
	output[1], err = yaml.Marshal(model.Bookmarks{Categories: next.Categories, Items: next.Bookmarks})
	if err != nil {
		return EditorDocument{}, err
	}
	type stagedFile struct {
		name, path, stage string
		original, next    []byte
		mode              os.FileMode
	}
	files := []*stagedFile{
		{name: "apps", original: raw[0], next: output[0]},
		{name: "bookmarks", original: raw[1], next: output[1]},
	}
	for _, file := range files {
		file.path = getConfigPath(file.name)
		info, statErr := os.Lstat(file.path)
		if statErr != nil {
			return EditorDocument{}, statErr
		}
		if !info.Mode().IsRegular() {
			return EditorDocument{}, fmt.Errorf("配置目标不是普通文件：%s", file.name)
		}
		file.mode = info.Mode().Perm()
		f, stageErr := os.CreateTemp(filepath.Dir(file.path), ".flare-editor-stage-*")
		if stageErr != nil {
			return EditorDocument{}, stageErr
		}
		file.stage = f.Name()
		defer func(path string) { _ = os.Remove(path) }(file.stage)
		stageErr = f.Chmod(file.mode)
		if stageErr == nil {
			_, stageErr = f.Write(file.next)
		}
		if stageErr == nil {
			stageErr = f.Sync()
		}
		closeErr := f.Close()
		if stageErr == nil {
			stageErr = closeErr
		}
		if stageErr != nil {
			return EditorDocument{}, stageErr
		}
	}
	// Disk versions are checked again after staging, so edits during preparation
	// cannot silently replace an already changed configuration.
	check, _, err := editorSnapshotLocked()
	if err != nil {
		return EditorDocument{}, err
	}
	if check.Version != next.Version {
		return EditorDocument{}, ErrBookmarkConflict
	}
	recoveryRoot := filepath.Join(filepath.Dir(getConfigPath("apps")), ".flare-editor-recovery")
	if err = os.MkdirAll(recoveryRoot, 0700); err != nil {
		return EditorDocument{}, err
	}
	info, err := os.Lstat(recoveryRoot)
	if err != nil {
		return EditorDocument{}, err
	}
	if !info.IsDir() {
		return EditorDocument{}, errors.New("恢复目录无效")
	}
	recovery, err := os.MkdirTemp(recoveryRoot, "save-")
	if err != nil {
		return EditorDocument{}, err
	}
	for _, file := range files {
		if err = atomicBookmarkWrite(filepath.Join(recovery, file.name+".yml"), file.original, 0600); err != nil {
			return EditorDocument{}, err
		}
	}
	// Keep completed replacements themselves, rather than indexing parallel
	// arrays. The rollback list contains only files actually replaced.
	var replaced []*stagedFile
	for _, file := range files {
		if err = rename(file.stage, file.path); err != nil {
			var restoreErrors []error
			for _, written := range replaced {
				if restoreErr := atomicBookmarkWrite(written.path, written.original, written.mode); restoreErr != nil {
					restoreErrors = append(restoreErrors, restoreErr)
				}
			}
			invalidateFileCache("apps")
			invalidateFileCache("bookmarks")
			if len(restoreErrors) > 0 {
				return EditorDocument{}, fmt.Errorf("保存失败且恢复未完成，可能存在部分写入；原文件保存在 %s：%w", recovery, errors.Join(restoreErrors...))
			}
			return EditorDocument{}, fmt.Errorf("保存失败，已恢复已替换文件；恢复副本保存在 %s：%w", recovery, err)
		}
		replaced = append(replaced, file)
	}
	invalidateFileCache("apps")
	invalidateFileCache("bookmarks")
	_ = os.RemoveAll(recovery)
	next.Version = editorVersion(output[0], output[1])
	return next, nil
}
