package config

import (
	"github.com/spf13/viper"
	"time"
)

func applyDefaults(v *viper.Viper) {
	v.SetDefault("app.env", "local")
	v.SetDefault("app.name", "tenex")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.shutdown_timeout", 30*time.Second)

	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.ssl_mode", "disable")

	v.SetDefault("redis.addr", "localhost:6379")

	v.SetDefault("s3.endpoint", "localhost:9000")

	v.SetDefault("scheduler.reminder_retry_interval", 30*time.Second)
	v.SetDefault("scheduler.max_retries", 3)

	v.SetDefault("observability.metrics_port", 9090)
}
