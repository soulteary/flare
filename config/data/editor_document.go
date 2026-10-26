package data

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/yamlutil"
	"go.yaml.in/yaml/v3"
)

// EditorDocument retains category IDs and private flags across every editor view.
// Versions cover the on-disk bytes of BOTH collections, including external edits.
type EditorDocument struct {
	RecoveryPaths []string         `json:"recoveryPaths,omitempty"`
	Version       string           `json:"version"`
	Categories    []model.Category `json:"categories"`
	Apps          []model.Bookmark `json:"apps"`
	Bookmarks     []model.Bookmark `json:"bookmarks"`
}

type EditorValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *EditorValidationError) Error() string { return e.Message }

// EditorSaveError identifies failed replacement and whether manual recovery is needed.
// RecoveryPath contains the original pair of files, not a new editor draft.
type EditorSaveError struct {
	RecoveryPath     string
	RecoveryRequired bool
	Cause            error
}

// Error retains an actionable recovery location for authenticated editor users.
func (e *EditorSaveError) Error() string {
	if e.RecoveryRequired {
		return fmt.Sprintf("保存失败且恢复未完成，可能存在部分写入；原文件保存在 %s：%v", e.RecoveryPath, e.Cause)
	}
	return fmt.Sprintf("保存失败，已恢复已替换文件；恢复副本保存在 %s：%v", e.RecoveryPath, e.Cause)
}

// Unwrap preserves the replacement failure and any rollback failures for logging.
func (e *EditorSaveError) Unwrap() error { return e.Cause }

func editorVersion(apps, bookmarks []byte) string {
	raw := append([]byte(fmt.Sprintf("%d:", len(apps))), apps...)
	raw = append(raw, bookmarks...)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func editorSnapshotLocked() (EditorDocument, [2][]byte, error) {
	var doc EditorDocument
	var raw [2][]byte
	recoveries, err := EditorRecoveryDirectories()
	if err != nil {
		return doc, raw, err
	}
	if len(recoveries) > 0 {
		for _, name := range []string{"apps", "bookmarks"} {
			if _, statErr := os.Stat(getConfigPath(name)); statErr != nil {
				return doc, raw, fmt.Errorf("live editor file %s is unavailable while recovery copies exist; inspect %s before restoring files: %w", name, recoveries[0], statErr)
			}
		}
	}
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
	if err := yamlutil.UnmarshalStrict(raw[0], &apps); err != nil {
		return doc, raw, err
	}
	if err := yamlutil.UnmarshalStrict(raw[1], &bookmarks); err != nil {
		return doc, raw, err
	}
	doc = EditorDocument{Version: editorVersion(raw[0], raw[1]), Categories: bookmarks.Categories, Apps: apps.Items, Bookmarks: bookmarks.Items}
	return doc, raw, nil
}

func EditorSnapshot() (EditorDocument, error) {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	doc, _, err := editorSnapshotLocked()
	if err != nil {
		return doc, err
	}
	doc.RecoveryPaths, err = EditorRecoveryDirectories()
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
	return validateEditorBookmarks(doc, ids)
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
	// Avoid rewriting untouched collections: YAML comments and formatting are not
	// represented by the editor document, so re-encoding would discard them.
	appsUnchanged := slices.Equal(next.Apps, before.Apps)
	bookmarksUnchanged := slices.Equal(next.Categories, before.Categories) && slices.Equal(next.Bookmarks, before.Bookmarks)
	if appsUnchanged && bookmarksUnchanged {
		return before, nil
	}
	output, err := encodeEditorFiles(next, raw, appsUnchanged, bookmarksUnchanged)
	if err != nil {
		return EditorDocument{}, err
	}
	files := []*stagedFile{
		{name: "apps", original: raw[0], next: output[0]},
		{name: "bookmarks", original: raw[1], next: output[1]},
	}
	defer cleanupEditorStages(files)
	for _, file := range files {
		if err = stageEditorFile(file); err != nil {
			return EditorDocument{}, err
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
	recovery, err := prepareEditorRecovery(files)
	if err != nil {
		return EditorDocument{}, err
	}
	if err = replaceEditorFiles(files, recovery, rename); err != nil {
		return EditorDocument{}, err
	}
	invalidateFileCache("apps")
	invalidateFileCache("bookmarks")
	_ = os.RemoveAll(recovery)
	next.Version = editorVersion(output[0], output[1])
	return next, nil
}

// EditorRecoveryDirectories reports retained recovery copies without trusting symlinks
// or silently overwriting either live collection. Old copies may follow a complete
// rollback, so presence alone cannot establish whether the live pair is consistent.
func EditorRecoveryDirectories() ([]string, error) {
	root := filepath.Join(filepath.Dir(getConfigPath("apps")), ".flare-editor-recovery")
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("editor recovery path is not a directory: %s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "save-") {
			paths = append(paths, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func validateEditorBookmarks(doc EditorDocument, ids map[string]bool) error {
	invalid := func(field, message string) error { return &EditorValidationError{field, message} }
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

type stagedFile struct {
	name, path, stage string
	original, next    []byte
	mode              os.FileMode
}

func stageEditorFile(file *stagedFile) error {
	file.path = getConfigPath(file.name)
	info, statErr := os.Lstat(file.path)
	if statErr != nil {
		return statErr
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("配置目标不是普通文件：%s", file.name)
	}
	file.mode = info.Mode().Perm()
	f, stageErr := os.CreateTemp(filepath.Dir(file.path), ".flare-editor-stage-*")
	if stageErr != nil {
		return stageErr
	}
	file.stage = f.Name()
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
		return stageErr
	}
	return nil
}

func prepareEditorRecovery(files []*stagedFile) (string, error) {
	recoveryRoot := filepath.Join(filepath.Dir(getConfigPath("apps")), ".flare-editor-recovery")
	if err := os.MkdirAll(recoveryRoot, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(recoveryRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("恢复目录无效")
	}
	recovery, err := os.MkdirTemp(recoveryRoot, "save-")
	if err != nil {
		return "", err
	}
	for _, file := range files {
		if err := atomicBookmarkWrite(filepath.Join(recovery, file.name+".yml"), file.original, 0600); err != nil {
			return "", err
		}
	}

	return recovery, nil
}

func replaceEditorFiles(files []*stagedFile, recovery string, rename func(string, string) error) error {
	// Keep completed replacements themselves, rather than indexing parallel
	// arrays. The rollback list contains only files actually replaced.
	var replaced []*stagedFile
	for _, file := range files {
		if err := rename(file.stage, file.path); err != nil {
			var restoreErrors []error
			for _, written := range replaced {
				if restoreErr := atomicBookmarkWrite(written.path, written.original, written.mode); restoreErr != nil {
					restoreErrors = append(restoreErrors, restoreErr)
				}
			}
			invalidateFileCache("apps")
			invalidateFileCache("bookmarks")
			if len(restoreErrors) > 0 {
				return &EditorSaveError{RecoveryPath: recovery, RecoveryRequired: true, Cause: errors.Join(append([]error{err}, restoreErrors...)...)}
			}
			return &EditorSaveError{RecoveryPath: recovery, Cause: err}
		}
		replaced = append(replaced, file)
	}
	return nil
}

func encodeEditorFiles(next EditorDocument, raw [2][]byte, appsUnchanged, bookmarksUnchanged bool) ([2][]byte, error) {
	output := [2][]byte{}
	var err error
	output[0], err = yaml.Marshal(model.Bookmarks{Items: next.Apps})
	if err != nil {
		return output, err
	}
	output[1], err = yaml.Marshal(model.Bookmarks{Categories: next.Categories, Items: next.Bookmarks})
	if err != nil {
		return output, err
	}
	if appsUnchanged {
		output[0] = raw[0]
	}
	if bookmarksUnchanged {
		output[1] = raw[1]
	}
	return output, nil
}

func cleanupEditorStages(files []*stagedFile) {
	for _, file := range files {
		if file.stage != "" {
			_ = os.Remove(file.stage)
		}
	}
}
