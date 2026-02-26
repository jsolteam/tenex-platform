package config

import (
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

func WatchFile(path string, onChange func()) {
	v := viper.New()
	v.SetConfigFile(path)

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Println("config file changed:", e.Name)
		onChange()
	})
}
