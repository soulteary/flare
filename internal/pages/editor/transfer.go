package editor

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/bookmarks"
	"github.com/soulteary/flare/internal/i18n"
	"go.yaml.in/yaml/v3"
)

const transferPath = "/editor/transfer"

var transferKey [32]byte
var transferKeyOnce sync.Once
var transferKeyErr error

type transferTicket struct {
	Raw        []byte            `json:"raw"`
	Format     string            `json:"format"`
	Collection string            `json:"collection"`
	Options    bookmarks.Options `json:"options"`
	Version    string            `json:"version"`
	User       string            `json:"user"`
	Expires    int64             `json:"expires"`
}

func registerTransfers(e *fiber.App) {
	e.Get(transferPath, auth.AuthRequired, func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).Render("transfer.html", fiber.Map{"Locale": editorLocale(), "HTMLLang": i18n.HTMLLang(editorLocale())})
	})
	e.Post(transferPath+"/preview", auth.AuthRequired, transferRequest, previewTransfer)
	e.Post(transferPath+"/commit", auth.AuthRequired, transferRequest, commitTransfer)
	e.Get(transferPath+"/export", auth.AuthRequired, exportTransfer)
	e.Get(transferPath+"/backup", auth.AuthRequired, backupTransfer)
}

// A custom header cannot be sent by cross-origin forms. No CORS access is granted.
func transferRequest(c fiber.Ctx) error {
	if c.Get("X-Flare-Transfer") != "1" || c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.Status(http.StatusForbidden).SendString(transferText("请从 Flare 管理页面操作"))
	}
	return c.Next()
}

func signTicket(ticket transferTicket) (string, error) {
	transferKeyOnce.Do(func() { _, transferKeyErr = rand.Read(transferKey[:]) })
	if transferKeyErr != nil {
		return "", transferKeyErr
	}
	raw, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, transferKey[:])
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func readTicket(token, user string) (transferTicket, error) {
	var ticket transferTicket
	transferKeyOnce.Do(func() { _, transferKeyErr = rand.Read(transferKey[:]) })
	if transferKeyErr != nil {
		return ticket, transferKeyErr
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ticket, fmt.Errorf("无效的预览凭据")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ticket, err
	}
	signature, err := hex.DecodeString(parts[1])
	if err != nil {
		return ticket, err
	}
	mac := hmac.New(sha256.New, transferKey[:])
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return ticket, fmt.Errorf("预览凭据无效，请重新预览")
	}
	if err = json.Unmarshal(raw, &ticket); err != nil {
		return ticket, err
	}
	if ticket.User != user || ticket.Expires <= time.Now().Unix() {
		return ticket, fmt.Errorf("预览已失效，请重新预览")
	}
	ticket.Options.Apps = ticket.Collection == "apps"
	return ticket, nil
}

func transferError(c fiber.Ctx, status int, err error) error {
	log.Printf("bookmark transfer failed: %v", err)
	return c.Status(status).JSON(map[string]string{"error": transferText(err.Error())})
}

func previewTransfer(c fiber.Ctx) error {
	// Keep the existing multipart parser and exact limits; the Fiber handler itself stays native.
	rawBody := c.BodyRaw()
	r := &http.Request{
		Method:        c.Method(),
		Header:        http.Header{"Content-Type": []string{c.Get("Content-Type")}},
		Body:          io.NopCloser(bytes.NewReader(rawBody)),
		ContentLength: int64(len(rawBody)),
	}
	r.Body = http.MaxBytesReader(nil, r.Body, bookmarks.MaxBytes+(64<<10))
	if err := r.ParseMultipartForm(bookmarks.MaxBytes + (64 << 10)); err != nil {
		return transferError(c, http.StatusBadRequest, fmt.Errorf("无法读取文件（最大 8 MiB）: %w", err))
	}
	if r.MultipartForm != nil {
		defer func() {
			if cleanupErr := r.MultipartForm.RemoveAll(); cleanupErr != nil {
				log.Printf("cleaning bookmark upload: %v", cleanupErr)
			}
		}()
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return transferError(c, http.StatusBadRequest, fmt.Errorf("请选择书签文件"))
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, bookmarks.MaxBytes+1))
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	format := r.FormValue("format")
	collection := r.FormValue("collection")
	options := bookmarks.Options{Strategy: r.FormValue("strategy"), Folder: r.FormValue("folder"), Flatten: r.FormValue("flatten") == "1", StripRoot: r.FormValue("stripRoot") == "1", AllowOther: r.FormValue("allowOther") == "1", Apps: collection == "apps"}
	entries, err := bookmarks.Parse(raw, format)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	current, version, err := data.BookmarkSnapshot(collection)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	plan, err := bookmarks.Build(current, entries, options)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	ticket := transferTicket{Raw: raw, Format: format, Collection: collection, Options: options, Version: version, User: auth.GetUserName(c), Expires: time.Now().Add(15 * time.Minute).Unix()}
	token, err := signTicket(ticket)
	if err != nil {
		return transferError(c, http.StatusInternalServerError, err)
	}
	localizeTransferPlan(&plan)
	c.Set("Cache-Control", "no-store")
	return c.Status(http.StatusOK).JSON(map[string]any{"plan": plan, "token": token})
}

func commitTransfer(c fiber.Ctx) error {
	bodyReader := http.MaxBytesReader(nil, io.NopCloser(bytes.NewReader(c.BodyRaw())), 16<<20)
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(bodyReader).Decode(&body); err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	ticket, err := readTicket(body.Token, auth.GetUserName(c))
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	entries, err := bookmarks.Parse(ticket.Raw, ticket.Format)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	current, version, err := data.BookmarkSnapshot(ticket.Collection)
	if err != nil {
		return transferError(c, http.StatusInternalServerError, err)
	}
	if version != ticket.Version {
		return transferError(c, http.StatusConflict, data.ErrBookmarkConflict)
	}
	plan, err := bookmarks.Build(current, entries, ticket.Options)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	if plan.Added+plan.Updated > 0 {
		if err = data.CommitBookmarkImport(ticket.Collection, ticket.Version, plan.Result); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, data.ErrBookmarkConflict) {
				status = http.StatusConflict
			}
			return transferError(c, status, err)
		}
	}
	localizeTransferPlan(&plan)
	c.Set("Cache-Control", "no-store")
	return c.Status(http.StatusOK).JSON(plan)
}

func transferDownload(c fiber.Ctx, name, mime string, raw []byte) error {
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Type", mime)
	return c.Status(http.StatusOK).Send(raw)
}

func exportTransfer(c fiber.Ctx) error {
	collection := c.Query("collection")
	current, _, err := data.BookmarkSnapshot(collection)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	switch c.Query("format") {
	case "yaml":
		raw, err := yaml.Marshal(current)
		if err != nil {
			return transferError(c, http.StatusInternalServerError, err)
		}
		return transferDownload(c, "flare-"+collection+".yml", "application/yaml", raw)
	case "html":
		raw, err := bookmarks.HTML(current)
		if err != nil {
			return transferError(c, http.StatusInternalServerError, err)
		}
		return transferDownload(c, "flare-"+collection+".html", "text/html; charset=utf-8", raw)
	default:
		return transferError(c, http.StatusBadRequest, fmt.Errorf("请选择 HTML 或 YAML"))
	}
}

func backupTransfer(c fiber.Ctx) error {
	collection := c.Query("collection")
	raw, when, err := data.LatestBookmarkBackup(collection)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		return transferError(c, status, fmt.Errorf("没有可用的备份或备份无法读取"))
	}
	return transferDownload(c, "flare-"+collection+"-before-import-"+when.UTC().Format("20060102T150405")+".yml", "application/yaml", raw)
}

// Translate validation messages at the HTTP boundary, preserving parser details
// and user-provided field values following a diagnostic prefix.
var transferMessageKeys = map[string]string{
	"无效的数据类型":                  "transfer.invalid_collection",
	"书签已发生变化，请重新预览":            "transfer.conflict",
	"请从 Flare 管理页面操作":          "transfer.management_required",
	"无效的预览凭据":                  "transfer.invalid_token",
	"预览凭据无效，请重新预览":             "transfer.invalid_signature",
	"预览已失效，请重新预览":              "transfer.preview_expired",
	"无法读取文件（最大 8 MiB）":         "transfer.file_read_error",
	"请选择书签文件":                  "transfer.choose_bookmark_file",
	"请选择 HTML 或 YAML":          "transfer.choose_export_format",
	"没有可用的备份或备份无法读取":           "transfer.backup_unavailable",
	"文件必须为 UTF-8，且不超过 8 MiB":   "transfer.invalid_encoding_or_size",
	"无效的 Flare YAML":           "transfer.invalid_yaml",
	"缺少 links 字段":              "transfer.missing_links",
	"分类 ID 或名称无效或重复":           "transfer.invalid_category",
	"书签引用了不存在的分类":              "transfer.unknown_category",
	"请选择 HTML 或 Flare YAML 格式": "transfer.choose_import_format",
	"书签 HTML 不完整或缺少 DL 目录":     "transfer.incomplete_html",
	"嵌套的书签链接":                  "transfer.nested_links",
	"目录结构不完整":                  "transfer.incomplete_folders",
	"链接为空或包含非法字符":              "transfer.invalid_url_characters",
	"链接格式无效":                   "transfer.invalid_url",
	"缺少域名":                     "transfer.missing_hostname",
	"不支持或不安全的协议":               "transfer.unsafe_protocol",
	"相对地址或其他协议需要手动允许":          "transfer.allow_other_required",
	"无效的重复处理策略":                "transfer.invalid_strategy",
	"书签数量超出限制":                 "transfer.bookmark_limit",
	"图标包含非法字符":                 "transfer.invalid_icon",
	"缺少名称，已使用域名或链接":            "transfer.missing_name",
	"最多支持 20000 个书签或分类":        "transfer.item_limit",
	"最多支持 20000 个书签":           "transfer.bookmark_count_limit",
	"目录深度不能超过 32":              "transfer.depth_limit",
	"合并后最多支持 20000 个书签或分类":     "transfer.merged_item_limit",
}

func transferText(source string) string {
	if key, ok := transferMessageKeys[source]; ok {
		return i18n.T(editorLocale(), key)
	}
	for prefix, key := range transferMessageKeys {
		if strings.HasPrefix(source, prefix+": ") {
			return i18n.T(editorLocale(), key) + strings.TrimPrefix(source, prefix)
		}
	}
	if source == "" {
		return ""
	}
	return i18n.T(editorLocale(), "transfer.unknown_error")
}

func localizeTransferPlan(plan *bookmarks.Plan) {
	for index := range plan.Rows {
		plan.Rows[index].Reason = transferText(plan.Rows[index].Reason)
	}
}
