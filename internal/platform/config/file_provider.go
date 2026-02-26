package config

import "github.com/spf13/viper"

type FileProvider struct {
	Path string
}

func (f *FileProvider) Name() string {
	return "file"
}

func (f *FileProvider) Load() (map[string]interface{}, error) {
	v := viper.New()
	v.SetConfigFile(f.Path)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v.AllSettings(), nil
}
