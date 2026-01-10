package logx

import "log/slog"

type SlogAdapter struct {
	l *slog.Logger
}

func NewSlogAdapter(l *slog.Logger) Logger {
	return &SlogAdapter{l: l}
}

func (s *SlogAdapter) Debug(msg string, fields ...Field) { s.l.Debug(msg, toSlogArgs(fields)...) }
func (s *SlogAdapter) Info(msg string, fields ...Field)  { s.l.Info(msg, toSlogArgs(fields)...) }
func (s *SlogAdapter) Warn(msg string, fields ...Field)  { s.l.Warn(msg, toSlogArgs(fields)...) }
func (s *SlogAdapter) Error(msg string, fields ...Field) { s.l.Error(msg, toSlogArgs(fields)...) }

func (s *SlogAdapter) With(fields ...Field) Logger {
	return &SlogAdapter{l: s.l.With(toSlogArgs(fields)...)}
}

func (s *SlogAdapter) Sync() error { return nil } // slog обычно noop

func toSlogArgs(fields []Field) []any {
	args := make([]any, 0, len(fields))
	for _, f := range fields {
		args = append(args, slog.Any(f.Key, f.Value))
	}
	return args
}
