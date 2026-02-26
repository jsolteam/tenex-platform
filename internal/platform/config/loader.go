package config

import (
	"github.com/spf13/viper"
)

type Loader struct {
	providers []Provider
}

func NewLoader(providers ...Provider) *Loader {
	return &Loader{providers: providers}
}

func (l *Loader) Load() (*AppConfig, error) {
	v := viper.New()

	// lowest priority first
	for i := len(l.providers) - 1; i >= 0; i-- {
		settings, err := l.providers[i].Load()
		if err != nil {
			continue
		}
		for k, val := range settings {
			v.Set(k, val)
		}
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
