package assets

import (
	"net"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Native in-process server dispatch: no load-generator/client allocations or
// network are included. These are allocation/latency microbenchmarks, not RSS.
func BenchmarkStaticResourceDispatch(b *testing.B) {
	for _, conditional := range []bool{false, true} {
		name := "full"
		if conditional {
			name = "conditional"
		}
		b.Run(name, func(b *testing.B) {
			app := fiber.New()
			app.Use(optimizeResourceCacheTime())
			app.Use("/assets/editor-ui", StaticFS(fstest.MapFS{"app.js": {Data: []byte(strings.Repeat("x", 256<<10))}}, "/assets/editor-ui"))
			handler := app.Handler()
			var request fasthttp.Request
			request.SetRequestURI("http://flare.test/assets/editor-ui/app.js")
			var ctx fasthttp.RequestCtx
			ctx.Init(&request, &net.TCPAddr{}, nil)
			handler(&ctx)
			if ctx.Response.StatusCode() != 200 {
				b.Fatal(ctx.Response.StatusCode())
			}
			etag := string(ctx.Response.Header.Peek("ETag"))
			if conditional {
				ctx.Request.Header.Set("If-None-Match", etag)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				ctx.Response.Reset()
				handler(&ctx)
				if conditional && ctx.Response.StatusCode() != 304 {
					b.Fatal(ctx.Response.StatusCode())
				}
			}
		})
	}
}
