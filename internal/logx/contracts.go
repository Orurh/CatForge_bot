package logx

import "time"

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Sync() error
}

type Field struct {
	Key   string
	Value any
}

func Any(key string, value any) Field                { return Field{Key: key, Value: value} }
func String(key, value string) Field                 { return Field{Key: key, Value: value} }
func Int(key string, value int) Field                { return Field{Key: key, Value: value} }
func Int64(key string, value int64) Field            { return Field{Key: key, Value: value} }
func Time(key string, value time.Time) Field         { return Field{Key: key, Value: value} }
func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }
