package core

import "go.uber.org/zap/zapcore"

func ParseLevel(l string) zapcore.Level {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(l)); err != nil {
		return zapcore.InfoLevel
	}
	return lvl
}
