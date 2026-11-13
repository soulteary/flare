package logger

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Hour, minute and second differ so a layout that repeats one field cannot pass.
func TestPrettyHandlerTimestampIncludesMinutes(t *testing.T) {
	at := time.Date(2026, time.October, 28, 13, 7, 42, 123_000_000, time.UTC)
	for _, tc := range []struct {
		name    string
		message string
		attrs   []slog.Attr
	}{
		{name: "message", message: "started"},
		{name: "attributes", message: "rejected", attrs: []slog.Attr{slog.Int("status", 403)}},
		{name: "empty message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			handler := NewPrettyHandler(&out, PrettyHandlerOptions{})
			record := slog.NewRecord(at, slog.LevelInfo, tc.message, 0)
			record.AddAttrs(tc.attrs...)

			require.NoError(t, handler.Handle(context.Background(), record))
			require.Regexp(t, `^\[13:07:42\.123\]`, out.String())
		})
	}
}
