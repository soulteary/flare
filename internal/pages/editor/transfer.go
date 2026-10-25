package editor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/bookmarks"
	"gopkg.in/yaml.v2"
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

func registerTransfers(e *echo.Echo) {
	e.GET(transferPath, func(c *echo.Context) error { return c.Render(http.StatusOK, "transfer.html", nil) }, auth.AuthRequired)
	e.POST(transferPath+"/preview", previewTransfer, auth.AuthRequired, transferRequest)
	e.POST(transferPath+"/commit", commitTransfer, auth.AuthRequired, transferRequest)
	e.GET(transferPath+"/export", exportTransfer, auth.AuthRequired)
	e.GET(transferPath+"/backup", backupTransfer, auth.AuthRequired)
}

// A custom header cannot be sent by cross-origin forms. No CORS access is granted.
func transferRequest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if c.Request().Header.Get("X-Flare-Transfer") != "1" || c.Request().Header.Get("Sec-Fetch-Site") == "cross-site" {
			return c.String(http.StatusForbidden, "请从 Flare 管理页面操作")
		}
		return next(c)
	}
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

func transferError(c *echo.Context, status int, err error) error {
	return c.JSON(status, map[string]string{"error": err.Error()})
}

func previewTransfer(c *echo.Context) error {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, bookmarks.MaxBytes+(64<<10))
	if err := r.ParseMultipartForm(bookmarks.MaxBytes + (64 << 10)); err != nil {
		return transferError(c, http.StatusBadRequest, fmt.Errorf("无法读取文件（最大 8 MiB）: %w", err))
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
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
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]any{"plan": plan, "token": token})
}

func commitTransfer(c *echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 16<<20)
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&body); err != nil {
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
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, plan)
}

func transferDownload(c *echo.Context, name, mime string, raw []byte) error {
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.Blob(http.StatusOK, mime, raw)
}

func exportTransfer(c *echo.Context) error {
	collection := c.QueryParam("collection")
	current, _, err := data.BookmarkSnapshot(collection)
	if err != nil {
		return transferError(c, http.StatusBadRequest, err)
	}
	switch c.QueryParam("format") {
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

func backupTransfer(c *echo.Context) error {
	collection := c.QueryParam("collection")
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
