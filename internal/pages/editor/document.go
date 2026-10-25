package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/auth"
)

func registerEditorDocument(e *fiber.App) {
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
	saved, err := data.CommitEditorDocument(doc)
	if err != nil {
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
	return c.Status(http.StatusInternalServerError).JSON(result)
}
