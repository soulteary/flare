package searchui

import (
	"crypto/sha256"
	"encoding/base64"
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderedScriptMatchesHash(t *testing.T) {
	tpl, err := template.New("page").Parse(Templates() + `<script>{{template "search-hybrid-script"}}</script>`)
	require.NoError(t, err)
	var out strings.Builder
	require.NoError(t, tpl.Execute(&out, nil))
	rendered := strings.TrimSuffix(strings.TrimPrefix(out.String(), "<script>"), "</script>")
	sum := sha256.Sum256([]byte(rendered))
	require.Equal(t, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'", Hash())
	require.NotContains(t, rendered, "innerHTML")
	require.NotContains(t, rendered, "eval(")
}
