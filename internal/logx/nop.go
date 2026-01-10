package logx

type nopLogger struct{}

func Nop() Logger { return nopLogger{} }

func (nopLogger) Debug(string, ...Field) {}
func (nopLogger) Info(string, ...Field)  {}
func (nopLogger) Warn(string, ...Field)  {}
func (nopLogger) Error(string, ...Field) {}

func (nopLogger) With(...Field) Logger { return nopLogger{} }
func (nopLogger) Sync() error          { return nil }

var _ Logger = nopLogger{}
