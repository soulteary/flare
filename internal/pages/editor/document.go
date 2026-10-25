package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{"error": "无法读取导航数据，请检查配置文件及日志。"})
	}
	return c.Status(http.StatusOK).JSON(doc)
}
func saveEditorDocument(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if c.Get("X-Flare-Editor") != "1" || c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.Status(http.StatusForbidden).JSON(map[string]string{"error": "请从 Flare 编辑器保存"})
	}
	var doc data.EditorDocument
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{"error": "提交数据格式无效"})
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{"error": "提交数据格式无效"})
	}
	saved, err := data.CommitEditorDocument(doc)
	if err != nil {
		var invalid *data.EditorValidationError
		if errors.As(err, &invalid) {
			return c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{"error": invalid.Message, "field": invalid.Field})
		}
		if errors.Is(err, data.ErrBookmarkConflict) {
			return c.Status(http.StatusConflict).JSON(map[string]string{"error": "服务器数据已变化，当前修改未保存。请导出待保存数据或重新载入。"})
		}
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{"error": err.Error()})
	}
	return c.Status(http.StatusOK).JSON(map[string]any{"saved": true, "document": saved})
}
