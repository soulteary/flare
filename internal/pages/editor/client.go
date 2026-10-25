package editor

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"sync"

	"github.com/gofiber/fiber/v3"
	resources "github.com/soulteary/flare/internal/resources/assets"
)

//go:embed client
var clientAssets embed.FS

func registerEditorClient(e *fiber.App) {
	assets, err := fs.Sub(clientAssets, "client")
	if err != nil {
		panic(err)
	}
	e.Use("/assets/editor-ui", resources.StaticFS(assets, "/assets/editor-ui"))
}

var editorVersionOnce sync.Once
var editorVersion string
var editorVersionErr error

// One content version covers both the default editor and its lazy batch assets.
// A new document therefore cannot reuse a previously fresh one-year cache entry
// from an older release, before the revalidation policy has reached that browser.
func editorAssetVersion() (string, error) {
	editorVersionOnce.Do(func() {
		hash := sha256.New()
		for _, files := range []fs.FS{clientAssets, editorAssets} {
			editorVersionErr = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				content, err := fs.ReadFile(files, name)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(hash, "%d:%s:%d:", len(name), name, len(content))
				_, _ = hash.Write(content)
				return nil
			})
			if editorVersionErr != nil {
				return
			}
		}
		editorVersion = fmt.Sprintf("%x", hash.Sum(nil))
	})
	return editorVersion, editorVersionErr
}
