// Package formvalue preserves net/http.Request.FormValue precedence on Fiber.
package formvalue

import (
	"mime"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// Get returns the first value for key, including an explicitly empty value.
// URL-encoded POST/PUT/PATCH bodies take precedence over query parameters;
// query parameters take precedence over multipart values. Returned strings own
// their storage and can safely outlive the request. Parse errors are ignored,
// as with net/http.Request.FormValue.
func Get(c fiber.Ctx, key string) string {
	request := c.Request()
	contentType := string(request.Header.ContentType())
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && (mediaType == fiber.MIMEApplicationForm || mediaType == fiber.MIMEMultipartForm) {
		// fasthttp's parsers match the type and boundary parameter name
		// case-sensitively. MIME parameter values, especially boundaries,
		// retain their original case when formatted.
		if normalized := mime.FormatMediaType(mediaType, params); normalized != contentType {
			request.Header.SetContentType(normalized)
		}
	}
	method := c.Method()
	if err == nil && mediaType == fiber.MIMEApplicationForm &&
		(method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
		if args := request.PostArgs(); args.Has(key) {
			return string(args.Peek(key))
		}
	}
	if args := request.URI().QueryArgs(); args.Has(key) {
		return string(args.Peek(key))
	}
	if err == nil && mediaType == fiber.MIMEMultipartForm {
		form, parseErr := request.MultipartFormWithLimit(c.App().Config().BodyLimit)
		if parseErr == nil {
			if values := form.Value[key]; len(values) > 0 {
				return values[0]
			}
		}
	}
	return ""
}
