package metrics

import "context"

const (
	LabelRepo   = "repo"
	LabelCode   = "code"
	LabelMethod = "method"
)

// DBMetrics — инструменты для измерения производительности и ошибок репозиториев.
//
// Регистрируются при старте приложения через NewDBMetrics(reg).
// Все репозитории разделяют один экземпляр.
//
// Метрики:
//
//	tenex_db_query_duration_seconds{repo, method}   — latency каждого запроса
//	tenex_db_errors_total{repo, method, code}       — счётчик ошибок БД по коду
type DBMetrics struct {
	QueryDuration Histogram
	QueryErrors   Counter
}

// NewDBMetrics регистрирует оба инструмента в переданном Registry.
func NewDBMetrics(reg Registry) (*DBMetrics, error) {
	dur, err := reg.Histogram(
		"tenex_db_query_duration_seconds",
		"Latency of repository DB calls, in seconds.",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		LabelRepo, LabelMethod,
	)
	if err != nil {
		return nil, err
	}

	errs, err := reg.Counter(
		"tenex_db_errors_total",
		"Total number of repository DB errors, by repo, method and error code.",
		LabelRepo, LabelMethod, LabelCode,
	)
	if err != nil {
		return nil, err
	}

	return &DBMetrics{
		QueryDuration: dur,
		QueryErrors:   errs,
	}, nil
}

// RecordDuration записывает latency запроса в секундах.
// Вызывается через defer в начале каждого метода репозитория:
//
//	start := time.Now()
//	defer r.met.RecordDuration(ctx, "user", "GetByID", start)
func (m *DBMetrics) RecordDuration(ctx context.Context, repo, method string, elapsedSec float64) {
	m.QueryDuration.Record(ctx, elapsedSec,
		L(LabelRepo, repo),
		L(LabelMethod, method),
	)
}

// RecordError увеличивает счётчик ошибок для данного репозитория, метода и кода.
//
//	m.met.RecordError(ctx, "user", "GetByID", string(appErr.Code))
func (m *DBMetrics) RecordError(ctx context.Context, repo, method, code string) {
	m.QueryErrors.Inc(ctx,
		L(LabelRepo, repo),
		L(LabelMethod, method),
		L(LabelCode, code),
	)
}
