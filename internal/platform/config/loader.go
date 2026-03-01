package config

import (
	"fmt"
	"os"
	"reflect"

	"github.com/spf13/viper"
)

type RequiredProvider interface {
	Provider
	IsRequired() bool
}

type MergeProvider interface {
	Provider
	UseDeepMerge() bool
}

type Loader struct {
	providers []Provider
}

func NewLoader(providers ...Provider) *Loader {
	return &Loader{providers: providers}
}

func (l *Loader) Load() (*AppConfig, error) {
	v := viper.New()
	applyDefaults(v)

	for _, p := range l.providers {
		settings, err := p.Load()
		if err != nil {
			required := false
			if rp, ok := p.(RequiredProvider); ok {
				required = rp.IsRequired()
			}
			if required {
				return nil, fmt.Errorf("required config provider %q failed: %w", p.Name(), err)
			}
			fmt.Fprintf(os.Stderr, "[config] optional provider %q skipped: %v\n", p.Name(), err)
			continue
		}

		if mp, ok := p.(MergeProvider); ok && mp.UseDeepMerge() {
			if err := v.MergeConfigMap(settings); err != nil {
				fmt.Fprintf(os.Stderr, "[config] deep merge failed for %q: %v\n", p.Name(), err)
			}
			continue
		}

		for k, val := range settings {
			if isEmptyValue(val) {
				continue
			}
			v.Set(k, val)
		}
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func isEmptyValue(val interface{}) bool {
	if val == nil {
		return true
	}
	rv := reflect.ValueOf(val)
	return rv.Kind() == reflect.String && rv.String() == ""
}
