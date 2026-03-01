package core

import "go.uber.org/zap"

type Logger struct {
	z *zap.Logger
}

func New(z *zap.Logger) *Logger {
	return &Logger{z: z}
}

func NewNoop() *Logger {
	return &Logger{z: zap.NewNop()}
}

func (l *Logger) Zap() *zap.Logger { return l.z }

func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{z: l.z.With(fields...)}
}

func (l *Logger) Info(msg string, fields ...zap.Field)  { l.z.Info(msg, fields...) }
func (l *Logger) Error(msg string, fields ...zap.Field) { l.z.Error(msg, fields...) }
func (l *Logger) Debug(msg string, fields ...zap.Field) { l.z.Debug(msg, fields...) }
func (l *Logger) Warn(msg string, fields ...zap.Field)  { l.z.Warn(msg, fields...) }
func (l *Logger) Fatal(msg string, fields ...zap.Field) { l.z.Fatal(msg, fields...) }

func (l *Logger) Sync() error { return l.z.Sync() }
