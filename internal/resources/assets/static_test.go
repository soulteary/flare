package assets

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type countedStaticFS struct {
	files fs.FS
	opens atomic.Int64
	reads atomic.Int64
}

func (f *countedStaticFS) Open(name string) (fs.File, error) {
	f.opens.Add(1)
	file, err := f.files.Open(name)
	if err != nil {
		return nil, err
	}
	return &countedStaticFile{File: file, reads: &f.reads}, nil
}

type countedStaticFile struct {
	fs.File
	reads *atomic.Int64
}

func (f *countedStaticFile) Read(p []byte) (int, error) { f.reads.Add(1); return f.File.Read(p) }
func (f *countedStaticFile) Seek(offset int64, whence int) (int64, error) {
	seeker, ok := f.File.(io.Seeker)
	if !ok {
		return 0, fs.ErrInvalid
	}
	return seeker.Seek(offset, whence)
}

func TestStaticMetadataAvoidsWarmConditionalAndHeadReads(t *testing.T) {
	files := &countedStaticFS{files: fstest.MapFS{"app.js": {Data: []byte("abcdef")}}}
	app := fiber.New()
	app.Use(optimizeResourceCacheTime())
	app.Use("/assets/editor-ui", StaticFS(files, "/assets/editor-ui"))
	request := func(method, etag, byteRange string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/assets/editor-ui/app.js", nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		rec := httptest.NewRecorder()
		serveHTTP(app, rec, req)
		return rec
	}
	first := request(http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, first.Code)
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	files.opens.Store(0)
	files.reads.Store(0)
	conditional := request(http.MethodGet, "W/"+etag, "")
	require.Equal(t, http.StatusNotModified, conditional.Code)
	head := request(http.MethodHead, "", "")
	require.Equal(t, http.StatusOK, head.Code)
	require.Equal(t, "6", head.Header().Get("Content-Length"))
	require.Empty(t, head.Body.String())
	require.Zero(t, files.opens.Load(), "metadata-only responses must not reopen the embedded file")
	require.Zero(t, files.reads.Load(), "metadata-only responses must not reread or hash its bytes")
	partial := request(http.MethodGet, "", "bytes=2-4")
	require.Equal(t, http.StatusPartialContent, partial.Code)
	require.Equal(t, "cde", partial.Body.String())
	require.EqualValues(t, 1, files.opens.Load())
	missing := httptest.NewRecorder()
	serveHTTP(app, missing, httptest.NewRequest(http.MethodGet, "/assets/editor-ui/missing.js", nil))
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Empty(t, missing.Header().Get("ETag"))
	require.Equal(t, "no-store", missing.Header().Get("Cache-Control"))
}

// Some fs.FS implementations have no seekable handles; full and range responses
// must still work without asking callers to implement an extra interface.
type nonSeekingStaticFS struct{ files fs.FS }

func (f nonSeekingStaticFS) Open(name string) (fs.File, error) {
	file, err := f.files.Open(name)
	if err != nil {
		return nil, err
	}
	return struct{ fs.File }{file}, nil
}
func TestStaticFSNonSeekableFallback(t *testing.T) {
	app := fiber.New()
	app.Use("/assets/editor-ui", StaticFS(nonSeekingStaticFS{fstest.MapFS{"app.js": {Data: []byte("abcdef")}}}, "/assets/editor-ui"))
	req := httptest.NewRequest(http.MethodGet, "/assets/editor-ui/app.js", nil)
	req.Header.Set("Range", "bytes=3-5")
	rec := httptest.NewRecorder()
	serveHTTP(app, rec, req)
	require.Equal(t, http.StatusPartialContent, rec.Code)
	require.Equal(t, "def", rec.Body.String())
}

func TestStaticMetadataOwnsKeysAcrossReusedNativeURIStorage(t *testing.T) {
	fixtures := fstest.MapFS{}
	for i := range 128 {
		fixtures[fmt.Sprintf("%03d.js", i)] = &fstest.MapFile{Data: []byte(fmt.Sprintf("script-%03d", i))}
	}
	files := &countedStaticFS{files: fixtures}
	app := fiber.New(fiber.Config{Immutable: false})
	app.Use("/assets/editor-ui", StaticFS(files, "/assets/editor-ui"))
	handler := app.Handler()
	var request fasthttp.Request
	request.SetRequestURI("http://flare.test/assets/editor-ui/000.js")
	var ctx fasthttp.RequestCtx
	ctx.Init(&request, &net.TCPAddr{}, nil)
	etags := make([]string, 128)
	for i := range 128 {
		ctx.Request.SetRequestURI(fmt.Sprintf("http://flare.test/assets/editor-ui/%03d.js", i))
		ctx.Response.Reset()
		handler(&ctx)
		require.Equal(t, http.StatusOK, ctx.Response.StatusCode())
		etags[i] = string(ctx.Response.Header.Peek("ETag"))
	}
	files.opens.Store(0)
	files.reads.Store(0)
	for i := 127; i >= 0; i-- {
		ctx.Request.SetRequestURI(fmt.Sprintf("http://flare.test/assets/editor-ui/%03d.js", i))
		ctx.Request.Header.Set("If-None-Match", etags[i])
		ctx.Response.Reset()
		handler(&ctx)
		require.Equal(t, http.StatusNotModified, ctx.Response.StatusCode(), "file %d", i)
	}
	require.Zero(t, files.opens.Load(), "URI reuse must not mutate retained metadata keys")
	require.Zero(t, files.reads.Load())
}
