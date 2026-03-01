package config

import "time"

type AppConfig struct {
	App           App
	DB            DB
	Redis         Redis
	Clients       Clients
	S3            S3
	Scheduler     Scheduler
	Observability Observability
}

type App struct {
	Env  string
	Name string
}

type DB struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type Redis struct {
	Addr string
}

type Clients struct {
	Telegram Messenger
}

type Messenger struct {
	Token string
}

type S3 struct {
	Endpoint string
	Key      string
	Secret   string
	Bucket   string
}

type Scheduler struct {
	ReminderRetryInterval time.Duration
	MaxRetries            int
}

type Observability struct {
	OTLPEndpoint string
}
