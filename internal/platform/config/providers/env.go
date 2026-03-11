package providers

import "os"

var envBindings = []struct {
	viperKey string
	envVar   string
}{
	{"app.env", "APP_ENV"},
	{"app.name", "APP_NAME"},
	{"app.log_level", "APP_LOG_LEVEL"},
	{"app.shutdown_timeout", "APP_SHUTDOWN_TIMEOUT"},
	{"db.host", "DB_HOST"},
	{"db.port", "DB_PORT"},
	{"db.user", "DB_USER"},
	{"db.password", "DB_PASS"},
	{"db.name", "DB_NAME"},
	{"db.ssl_mode", "DB_SSL_MODE"},
	{"redis.addr", "REDIS_ADDR"},
	{"s3.endpoint", "S3_ENDPOINT"},
	{"s3.key", "S3_KEY"},
	{"s3.secret", "S3_SECRET"},
	{"s3.bucket", "S3_BUCKET"},
	{"observability.otlp_endpoint", "OTEL_ENDPOINT"},
	{"observability.loki_endpoint", "LOKI_ENDPOINT"},
	{"observability.metrics_port", "METRICS_PORT"},
	{"clients.telegram.token", "TELEGRAM_TOKEN"},
	{"scheduler.max_retries", "SCHEDULER_MAX_RETRIES"},
	{"scheduler.reminder_retry_interval", "SCHEDULER_RETRY_INTERVAL"},
}

type EnvProvider struct{}

func (e *EnvProvider) Name() string { return "env" }

func (e *EnvProvider) IsRequired() bool { return false }

func (e *EnvProvider) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(envBindings))
	for _, b := range envBindings {
		if val := os.Getenv(b.envVar); val != "" {
			result[b.viperKey] = val
		}
	}
	return result, nil
}
