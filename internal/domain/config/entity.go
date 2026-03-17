package config

import "time"

// PlatformConfig — запись конфигурации платформы из БД.
type PlatformConfig struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}
