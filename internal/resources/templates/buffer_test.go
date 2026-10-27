package templates

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateBufferRetentionDropsOversizedBacking(t *testing.T) {
	oversized := bytes.NewBufferString(strings.Repeat("x", 2*maxRetainedTemplateBuffer))
	require.Greater(t, oversized.Cap(), maxRetainedTemplateBuffer)
	releaseTemplateBuffer(oversized)
	require.Zero(t, oversized.Len())
	require.Zero(t, oversized.Cap(), "pooling the object must not retain a large HTML backing allocation")
	small := bytes.NewBufferString("ordinary page")
	capacity := small.Cap()
	releaseTemplateBuffer(small)
	require.Zero(t, small.Len())
	require.Equal(t, capacity, small.Cap(), "ordinary pages should remain eligible for buffer reuse")
}

func TestRendererKeepsTemplateFailureAtomic(t *testing.T) {
	failure := errors.New("template execution failed")
	parsed := template.Must(template.New("page").Parse(`prefix{{call .Fail}}suffix`))
	renderer := &Renderer{templates: parsed}
	var output bytes.Buffer
	output.WriteString("already-written")
	err := renderer.Render(&output, "page", map[string]any{"Fail": func() (string, error) { return "", failure }})
	require.ErrorContains(t, err, failure.Error())
	require.Equal(t, "already-written", output.String(), "a failed template must not release its partially rendered prefix")
}

type failingTemplateWriter struct{ err error }

func (w failingTemplateWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRendererPreservesLargeOutputAndWriterErrors(t *testing.T) {
	parsed := template.Must(template.New("page").Parse(`<p>{{.}}</p>`))
	renderer := &Renderer{templates: parsed}
	large := strings.Repeat("navigation ", maxRetainedTemplateBuffer/3)
	var output bytes.Buffer
	require.NoError(t, renderer.Render(&output, "page", large))
	require.Equal(t, "<p>"+large+"</p>", output.String())
	failure := errors.New("writer unavailable")
	require.ErrorIs(t, renderer.Render(failingTemplateWriter{failure}, "page", "failed write"), failure)
	output.Reset()
	require.NoError(t, renderer.Render(&output, "page", "next & response"))
	require.Equal(t, "<p>next &amp; response</p>", output.String(), "a subsequent render must not contain a failed or oversized response")
}
