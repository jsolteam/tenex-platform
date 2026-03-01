package config

type Provider interface {
	Name() string
	Load() (map[string]interface{}, error)
}
