package observability

import (
	"context"
	"log/slog"
)

type LogHandler struct{ slog.Handler }

func (h LogHandler) Handle(ctx context.Context, r slog.Record) error {
	level := "info"
	switch {
	case r.Level >= slog.LevelError:
		level = "error"
	case r.Level >= slog.LevelWarn:
		level = "warn"
	case r.Level < slog.LevelInfo:
		level = "debug"
	}
	Logs.WithLabelValues(level).Inc()
	return h.Handler.Handle(ctx, r)
}
func (h LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogHandler{h.Handler.WithAttrs(attrs)}
}
func (h LogHandler) WithGroup(name string) slog.Handler { return LogHandler{h.Handler.WithGroup(name)} }
