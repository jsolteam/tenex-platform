package config

type Provider interface {
	Load() (map[string]interface{}, error)
	Name() string
}
