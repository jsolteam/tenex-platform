package core

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type BuildOptions struct {
	AtomicLevel zap.AtomicLevel
	ExtraCores  []zapcore.Core
	Async       bool
	QueueSize   int
	Sampling    bool
}

func Build(opts BuildOptions) (*Logger, func() error) {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	enc := zapcore.NewJSONEncoder(encCfg)
	consoleSink := zapcore.AddSync(os.Stdout)

	var consoleCore zapcore.Core = zapcore.NewCore(enc, consoleSink, opts.AtomicLevel)

	if opts.Sampling {
		consoleCore = NewSampling(consoleCore)
	}

	cores := []zapcore.Core{consoleCore}
	cores = append(cores, opts.ExtraCores...)

	var tee zapcore.Core = zapcore.NewTee(cores...)

	var asyncCore *AsyncCore
	if opts.Async {
		asyncCore = NewAsyncCore(tee, opts.QueueSize)
		tee = asyncCore
	}

	z := zap.New(tee,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	cleanup := func() error {
		if asyncCore != nil {
			return asyncCore.Close()
		}
		return z.Sync()
	}

	return New(z), cleanup
}
