package editor

import (
	"embed"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"io/fs"
)

//go:embed client
var clientAssets embed.FS

func registerEditorClient(e *fiber.App) {
	assets, err := fs.Sub(clientAssets, "client")
	if err != nil {
		panic(err)
	}
	e.Use("/assets/editor-ui", static.New("", static.Config{FS: assets}))
}
