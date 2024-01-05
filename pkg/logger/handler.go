package logger

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"

	"github.com/fatih/color"
)

type PrettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
}

type PrettyHandler struct {
	slog.Handler
	l *log.Logger
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.BlueString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	timeStr := r.Time.Format("[15:05:05.000]")
	msg := color.CyanString(r.Message)

	var b []byte
	var err error

	if r.NumAttrs() == 0 {
		if r.Message == "" {
			h.l.Println(timeStr)
		} else {
			h.l.Println(timeStr, level, msg)
		}
	} else {
		fields := make(map[string]interface{}, r.NumAttrs())
		r.Attrs(func(a slog.Attr) bool {
			fields[a.Key] = a.Value.Any()
			return true
		})
		b, err = json.Marshal(fields)
		// b, err = json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return err
		}
		h.l.Println(timeStr, level, msg, color.WhiteString(string(b)))
	}
	return nil
}

func NewPrettyHandler(out io.Writer, opts PrettyHandlerOptions) *PrettyHandler {
	h := &PrettyHandler{
		Handler: slog.NewJSONHandler(out, &opts.SlogOpts),
		l:       log.New(out, "", 0),
	}
	return h
}
