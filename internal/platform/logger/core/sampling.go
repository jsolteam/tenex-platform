package core

import "go.uber.org/zap/zapcore"

func NewSampling(inner zapcore.Core) zapcore.Core {
	return zapcore.NewSamplerWithOptions(inner, 100, 10, 5)
}
