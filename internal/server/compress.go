package server

import (
	"bytes"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"

	"github.com/soulteary/flare/internal/auth"
)

// compressMinimum is the size below which a response is sent as it is: the
// header of a coding and the work cost more than they save.
const compressMinimum = 1024

type plainETagKey struct{}

// compressible lists the media types whose bytes a coding shortens. Icons in
// ICO, raster images and archives are already packed.
var compressible = []string{
	"text/",
	"application/javascript",
	"application/json",
	"application/manifest+json",
	"application/xml",
	"image/svg+xml",
}

// registerCompression sends a response in a coding the client accepts.
//
// Three handlers share the work, from the outside in. The first restores what
// the others changed for their own use. The second is Fiber's, which codes
// every response that has no coding yet. The third runs right after the route
// and names the coding "identity" for a response that must stay as it is, so
// that the second leaves it alone.
//
// A response stays as it is when it is small or of a packed type, and when it can hold a secret of the visitor: with a compressed body the
// length tells how much of that secret a chosen part of the request repeats.
// A response that names itself public holds none.
func registerCompression(e *fiber.App) {
	e.Use(func(c fiber.Ctx) error {
		err := c.Next()
		ownVaryLine(c)
		coding := c.GetRespHeader(fiber.HeaderContentEncoding)
		if coding == "identity" {
			c.Response().Header.Del(fiber.HeaderContentEncoding)
			return err
		}
		// One validator names the file whatever its coding, so a client that
		// kept a coded copy can ask whether the file changed. A weak validator
		// says the bytes may differ.
		if tag, ok := c.Locals(plainETagKey{}).(string); ok && coding != "" {
			c.Set(fiber.HeaderETag, "W/"+tag)
		}
		return err
	})
	e.Use(compress.New())
	e.Use(func(c fiber.Ctx) error {
		err := c.Next()
		if err != nil {
			return err
		}
		if c.GetRespHeader(fiber.HeaderContentEncoding) != "" {
			return nil
		}
		if !mayCompress(c) {
			c.Set(fiber.HeaderContentEncoding, "identity")
			return nil
		}
		if tag := c.GetRespHeader(fiber.HeaderETag); tag != "" && !strings.HasPrefix(tag, "W/") {
			c.Locals(plainETagKey{}, tag)
		}
		return nil
	})
}

func mayCompress(c fiber.Ctx) bool {
	response := c.Response()
	size := response.Header.ContentLength()
	if !response.IsBodyStream() {
		size = len(response.Body())
	}
	if size < compressMinimum {
		return false
	}
	kind := bytes.ToLower(response.Header.ContentType())
	packed := true
	for _, prefix := range compressible {
		if bytes.HasPrefix(kind, []byte(prefix)) {
			packed = false
			break
		}
	}
	if packed {
		return false
	}
	for _, directive := range strings.Split(c.GetRespHeader(fiber.HeaderCacheControl), ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "public") {
			return true
		}
	}
	return !auth.CarriesVisitorSecret(c)
}

// ownVaryLine gives Accept-Encoding a line of its own in Vary. Fiber appends
// it to a line that is there, which changes a line a route wrote: a reader
// that looks for the line "Cookie" would no longer find it.
func ownVaryLine(c fiber.Ctx) {
	header := &c.Response().Header
	var kept []string
	found := false
	for _, line := range header.PeekAll(fiber.HeaderVary) {
		var names []string
		for _, name := range strings.Split(string(line), ",") {
			name = strings.TrimSpace(name)
			switch {
			case name == "":
			case strings.EqualFold(name, fiber.HeaderAcceptEncoding):
				found = true
			default:
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			kept = append(kept, strings.Join(names, ", "))
		}
	}
	if !found {
		return
	}
	header.Del(fiber.HeaderVary)
	for _, line := range kept {
		header.Add(fiber.HeaderVary, line)
	}
	header.Add(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
}
