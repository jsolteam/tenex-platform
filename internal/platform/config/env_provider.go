package config

import "github.com/spf13/viper"

type EnvProvider struct{}

func (e *EnvProvider) Name() string {
	return "env"
}

func (e *EnvProvider) Load() (map[string]interface{}, error) {
	v := viper.New()
	v.AutomaticEnv()

	return v.AllSettings(), nil
}
