package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/security"
	"github.com/soulteary/flare/internal/yamlutil"
)

func registerEditorDocument(e *fiber.App) {
	e.Get("/editor/csrf", auth.AuthRequired, func(c fiber.Ctx) error {
		c.Set("Cache-Control", "private, no-store")
		return c.JSON(fiber.Map{"csrfToken": auth.CSRFToken(c)})
	})
	e.Get("/editor/document", auth.AuthRequired, getEditorDocument)
	e.Post("/editor/document", auth.AuthRequired, saveEditorDocument)
}
func getEditorDocument(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	doc, err := data.EditorSnapshot()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{"error": editorText(editorLocale(), "无法读取导航数据，请检查配置文件及日志。")})
	}
	return c.Status(http.StatusOK).JSON(doc)
}
func saveEditorDocument(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if auth.PersistentSecurity(c) && !auth.ValidateCSRF(c) {
		return c.SendStatus(http.StatusForbidden)
	}
	if c.Get("X-Flare-Editor") != "1" || c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.Status(http.StatusForbidden).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), "请从 Flare 编辑器保存")})
	}
	var doc data.EditorDocument
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), "提交数据格式无效")})
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return c.Status(http.StatusBadRequest).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), "提交数据格式无效")})
	}
	before, err := data.EditorSnapshot()
	if err != nil {
		return editorSaveFailure(c, err)
	}
	if define.AppFlags.DisableLoginMode && documentPrivate(doc) {
		return c.Status(409).JSON(fiber.Map{"saved": false, "error": "Private content requires login / 私有内容需要先开启登录"})
	}
	sensitive := auth.PersistentSecurity(c) && !define.AppFlags.DisableLoginMode && documentRelaxed(before, doc)
	if sensitive {
		if !auth.RecentAuthentication(c) {
			return auth.RequireRecentAuthentication(c)
		}
		if c.Get("X-Flare-Confirm-Public") != "1" {
			return c.Status(409).JSON(fiber.Map{"code": "PUBLIC_CONFIRMATION_REQUIRED", "error": "Confirm public visibility / 请确认公开可见"})
		}
	}
	var saved data.EditorDocument
	err = auth.AuthorizedWrite(c, func() error {
		var commitErr error
		saved, commitErr = data.CommitEditorDocumentChecked(doc, func() error {
			if sensitive && !auth.RecentAuthentication(c) {
				return fiber.NewError(403, "REAUTH_REQUIRED")
			}
			if !auth.Revalidate(c) {
				return fmt.Errorf("session revoked")
			}
			return nil
		})
		return commitErr
	})
	if err != nil {
		if errors.Is(err, security.ErrInvalidSession) {
			return c.Status(401).JSON(fiber.Map{"code": "AUTH_REQUIRED"})
		}
		var httpErr *fiber.Error
		if errors.As(err, &httpErr) {
			return err
		}
		var invalid *data.EditorValidationError
		if errors.As(err, &invalid) {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), invalid.Message), "field": invalid.Field})
		}
		if errors.Is(err, data.ErrBookmarkConflict) {
			return c.Status(http.StatusConflict).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), "服务器数据已变化，当前修改未保存。请导出待保存数据或重新载入。")})
		}
		return editorSaveFailure(c, err)
	}
	return c.Status(http.StatusOK).JSON(map[string]any{"saved": true, "document": saved})
}

// editorSaveFailure distinguishes retryable rollback from manual recovery.
// Recovery metadata is only returned behind the editor authentication boundary.
func editorSaveFailure(c fiber.Ctx, err error) error {
	slog.Error("Editor document save failed", "error", err)
	result := map[string]any{"saved": false, "code": "save_failed", "error": editorText(editorLocale(), "保存失败，请检查服务端日志后重试。")}
	var recovery *data.EditorSaveError
	if errors.As(err, &recovery) {
		result["error"] = editorText(editorLocale(), "保存失败，已恢复已替换文件；请检查服务端恢复副本。") + ": " + recovery.Cause.Error()
		result["recoveryPath"] = recovery.RecoveryPath
		result["recoveryRequired"] = recovery.RecoveryRequired
		if recovery.RecoveryRequired {
			result["code"] = "recovery_required"
			result["error"] = editorText(editorLocale(), "保存失败且恢复未完成，请检查服务端恢复副本。") + ": " + recovery.Cause.Error()
		}
	}
	if errors.Is(err, yamlutil.ErrBookmarkAliasesReadOnly) {
		result["code"] = "READ_ONLY"
		result["error"] = err.Error()
		return c.Status(503).JSON(result)
	}
	if errors.Is(err, data.ErrBookmarkReadOnly) || errors.Is(err, os.ErrPermission) {
		result["code"] = "READ_ONLY"
		return c.Status(http.StatusServiceUnavailable).JSON(result)
	}
	return c.Status(http.StatusInternalServerError).JSON(result)
}

func documentPrivate(doc data.EditorDocument) bool {
	for _, c := range doc.Categories {
		if c.Private {
			return true
		}
	}
	for _, items := range [][]model.Bookmark{doc.Apps, doc.Bookmarks} {
		for _, b := range items {
			if b.Private {
				return true
			}
		}
	}
	return false
}
func documentRelaxed(before, next data.EditorDocument) bool {
	for _, group := range []struct{ before, next model.Bookmarks }{{model.Bookmarks{Items: before.Apps}, model.Bookmarks{Items: next.Apps}}, {model.Bookmarks{Categories: before.Categories, Items: before.Bookmarks}, model.Bookmarks{Categories: next.Categories, Items: next.Bookmarks}}} {
		public := map[string]int{}
		for _, b := range data.FilterNavigation(group.before, false).Items {
			public[publicBookmarkKey(b)]++
		}
		for _, b := range data.FilterNavigation(group.next, false).Items {
			if public[publicBookmarkKey(b)] == 0 {
				return true
			}
			public[publicBookmarkKey(b)]--
		}
	}
	cats := map[string]bool{}
	for _, c := range before.Categories {
		cats[c.ID] = c.Private
	}
	for _, c := range next.Categories {
		wasPrivate, exists := cats[c.ID]
		if !c.Private && (!exists || wasPrivate) {
			return true
		}
	}
	return false
}

func publicBookmarkKey(b model.Bookmark) string {
	return fmt.Sprintf("%q/%q/%q/%q/%q", b.Category, b.URL, b.Name, b.Desc, b.Icon)
}
