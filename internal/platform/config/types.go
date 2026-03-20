package config

import "time"

type AppConfig struct {
	App           App           `mapstructure:"app"`
	DB            DB            `mapstructure:"db"`
	Redis         Redis         `mapstructure:"redis"`
	Clients       Clients       `mapstructure:"clients"`
	S3            S3            `mapstructure:"s3"`
	Scheduler     Scheduler     `mapstructure:"scheduler"`
	Observability Observability `mapstructure:"observability"`
}

type App struct {
	Env             string        `mapstructure:"env"`
	Name            string        `mapstructure:"name"`
	LogLevel        string        `mapstructure:"log_level"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DB struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Clients struct {
	Telegram Messenger `mapstructure:"telegram"`
}

type Messenger struct {
	Token string `mapstructure:"token"`
}

type S3 struct {
	Endpoint string `mapstructure:"endpoint"`
	Key      string `mapstructure:"key"`
	Secret   string `mapstructure:"secret"`
	Bucket   string `mapstructure:"bucket"`
}

type Scheduler struct {
	ReminderRetryInterval time.Duration `mapstructure:"reminder_retry_interval"`
	MaxRetries            int           `mapstructure:"max_retries"`
}

type Observability struct {
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`
	LokiEndpoint string `mapstructure:"loki_endpoint"`
	MetricsPort  int    `mapstructure:"metrics_port"`
}
