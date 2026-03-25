package metrics

import "context"

// S3Metrics — latency/error метрики для операций объектного хранилища.
type S3Metrics struct {
	OpDuration Histogram
	OpErrors   Counter
}

var s3DurationBuckets = []float64{
	0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5,
}

func NewS3Metrics(reg Registry) (*S3Metrics, error) {
	dur, err := reg.Histogram(
		"tenex_s3_op_duration_seconds",
		"Latency of S3 operations, in seconds.",
		s3DurationBuckets,
		LabelComponent, LabelMethod,
	)
	if err != nil {
		return nil, err
	}

	errs, err := reg.Counter(
		"tenex_s3_errors_total",
		"Total number of S3 errors, by component, method and error code.",
		LabelComponent, LabelMethod, LabelCode,
	)
	if err != nil {
		return nil, err
	}

	return &S3Metrics{OpDuration: dur, OpErrors: errs}, nil
}

func (m *S3Metrics) RecordDuration(ctx context.Context, component, method string, elapsedSec float64) {
	m.OpDuration.Record(ctx, elapsedSec,
		L(LabelComponent, component),
		L(LabelMethod, method),
	)
}

func (m *S3Metrics) RecordError(ctx context.Context, component, method, code string) {
	m.OpErrors.Inc(ctx,
		L(LabelComponent, component),
		L(LabelMethod, method),
		L(LabelCode, code),
	)
}
