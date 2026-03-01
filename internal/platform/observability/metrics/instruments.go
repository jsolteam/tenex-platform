package metrics

const (
	LabelMessenger = "messenger"
	LabelStatus    = "status"
	LabelHandler   = "handler"
	LabelState     = "state"
	LabelModule    = "module"
)

var processingDurationBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

type AppMetrics struct {
	UpdatesTotal Counter

	UpdateProcessingDuration Histogram

	FSMTransitionsTotal Counter

	RetryTotal Counter

	AdapterErrorsTotal Counter
}

func NewAppMetrics(reg Registry) (*AppMetrics, error) {
	updatesTotal, err := reg.Counter(
		"tenex_updates_total",
		"Total number of incoming messenger updates.",
		LabelMessenger, LabelStatus,
	)
	if err != nil {
		return nil, err
	}

	updateDuration, err := reg.Histogram(
		"tenex_update_processing_duration_seconds",
		"Time spent processing a single update, in seconds.",
		processingDurationBuckets,
		LabelMessenger, LabelHandler,
	)
	if err != nil {
		return nil, err
	}

	fsmTransitions, err := reg.Counter(
		"tenex_fsm_transitions_total",
		"Total number of FSM state transitions.",
		LabelState, LabelHandler,
	)
	if err != nil {
		return nil, err
	}

	retryTotal, err := reg.Counter(
		"tenex_retry_total",
		"Total number of retry attempts.",
		LabelModule, LabelStatus,
	)
	if err != nil {
		return nil, err
	}

	adapterErrors, err := reg.Counter(
		"tenex_adapter_errors_total",
		"Total number of errors reported by messenger adapters.",
		LabelMessenger, LabelStatus,
	)
	if err != nil {
		return nil, err
	}

	return &AppMetrics{
		UpdatesTotal:             updatesTotal,
		UpdateProcessingDuration: updateDuration,
		FSMTransitionsTotal:      fsmTransitions,
		RetryTotal:               retryTotal,
		AdapterErrorsTotal:       adapterErrors,
	}, nil
}
