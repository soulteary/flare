package assets

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

type staticMetadata struct {
	etag, contentType string
	size              int64
}

// staticContentType is bounded by Flare's bundled resource formats. Consulting
// mime.TypeByExtension even for a known favicon initializes the process-wide OS
// MIME registry, retaining host-dependent tables that these files do not need.
func staticContentType(name string, data []byte) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".ico":
		return "image/x-icon"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".json", ".map":
		return "application/json"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return http.DetectContentType(data)
	}
}

// StaticFS serves an immutable embedded filesystem. Metadata is prepared once per
// existing file; only small validators are retained, not another copy of its
// embedded bytes. Create a new handler whenever the underlying contents change.
// Warm conditional requests never open or read the resource, and HEAD/range
// behavior is still implemented by net/http.ServeContent.
func StaticFS(files fs.FS, prefix string) fiber.Handler {
	var mu sync.RWMutex
	metadata := make(map[string]staticMetadata)
	load := func(name string) (staticMetadata, error) {
		mu.RLock()
		info, ok := metadata[name]
		mu.RUnlock()
		if ok {
			return info, nil
		}
		mu.Lock()
		defer mu.Unlock()
		if info, ok = metadata[name]; ok {
			return info, nil
		}
		stat, err := fs.Stat(files, name)
		if err != nil {
			return info, err
		}
		if !stat.Mode().IsRegular() {
			return info, fs.ErrNotExist
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return info, err
		}
		contentType := staticContentType(name, data)
		info = staticMetadata{etag: fmt.Sprintf(`"%x"`, sha256.Sum256(data)), contentType: contentType, size: int64(len(data))}
		// Fiber can lend URI storage when Immutable is disabled. Only a cold
		// cache insertion needs an owned key; warmed lookups stay allocation-free.
		metadata[strings.Clone(name)] = info
		return info, nil
	}
	return func(c fiber.Ctx) error {
		if c.Method() != http.MethodGet && c.Method() != http.MethodHead {
			return c.Next()
		}
		name := strings.TrimPrefix(strings.TrimPrefix(c.Path(), prefix), "/")
		if !fs.ValidPath(name) || strings.ContainsRune(name, '\\') {
			return c.Next()
		}
		info, err := load(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return c.Next()
			}
			return err
		}
		reader := &lazyStaticReader{files: files, name: name, size: info.size}
		defer reader.close()
		// Only an existing public embedded resource can override the default
		// private policy. Missing files continue to the next handler unchanged.
		c.Set("Cache-Control", "public, max-age=0, must-revalidate")
		return serveReader(c, path.Base(name), info.contentType, info.etag, reader)
	}
}

// Seeking to obtain length does not need to open an immutable resource. The file
// is opened only if ServeContent actually needs body bytes (not HEAD or 304).
type lazyStaticReader struct {
	files        fs.FS
	name         string
	size, offset int64
	file         fs.File
	reader       io.ReadSeeker
}

func (r *lazyStaticReader) Read(p []byte) (int, error) {
	if r.reader == nil {
		file, err := r.files.Open(r.name)
		if err != nil {
			return 0, err
		}
		if seeker, ok := file.(io.ReadSeeker); ok {
			r.file = file
			r.reader = seeker
		} else {
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return 0, readErr
			}
			if closeErr != nil {
				return 0, closeErr
			}
			r.reader = bytes.NewReader(data)
		}
		if _, err = r.reader.Seek(r.offset, io.SeekStart); err != nil {
			return 0, err
		}
	}
	n, err := r.reader.Read(p)
	r.offset += int64(n)
	return n, err
}
func (r *lazyStaticReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.offset
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, errors.New("invalid seek origin")
	}
	if offset < 0 {
		return 0, errors.New("negative seek offset")
	}
	if r.reader != nil {
		if _, err := r.reader.Seek(offset, io.SeekStart); err != nil {
			return 0, err
		}
	}
	r.offset = offset
	return offset, nil
}
func (r *lazyStaticReader) close() {
	if r.file != nil {
		_ = r.file.Close()
	}
}
