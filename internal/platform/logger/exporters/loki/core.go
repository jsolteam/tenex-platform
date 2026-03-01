package loki

import (
	"go.uber.org/zap/zapcore"
)

type Core struct {
	writer  *Writer
	enc     zapcore.Encoder
	enabler zapcore.LevelEnabler
	fields  []zapcore.Field
}

var _ zapcore.Core = (*Core)(nil)

func NewCore(w *Writer, enabler zapcore.LevelEnabler) *Core {
	encCfg := zapcore.EncoderConfig{
		TimeKey:      "ts",
		LevelKey:     "level",
		MessageKey:   "msg",
		CallerKey:    "caller",
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}
	return &Core{
		writer:  w,
		enc:     zapcore.NewJSONEncoder(encCfg),
		enabler: enabler,
	}
}

func (c *Core) Enabled(l zapcore.Level) bool { return c.enabler.Enabled(l) } // H-5

func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	clone.fields = append(clone.fields, fields...)
	return clone
}

func (c *Core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return ce
}

func (c *Core) Write(e zapcore.Entry, fields []zapcore.Field) error {
	merged := make([]zapcore.Field, len(c.fields), len(c.fields)+len(fields))
	copy(merged, c.fields)
	merged = append(merged, fields...)

	buf, err := c.enc.EncodeEntry(e, merged)
	if err != nil {
		return err
	}
	_, err = c.writer.Write(buf.Bytes())
	buf.Free()
	return err
}

func (c *Core) Sync() error { return nil }

func (c *Core) clone() *Core {
	fields := make([]zapcore.Field, len(c.fields))
	copy(fields, c.fields)
	return &Core{
		writer:  c.writer,
		enc:     c.enc.Clone(),
		enabler: c.enabler,
		fields:  fields,
	}
}
