package providers

import (
	"github.com/spf13/viper"
)

// FileProvider reads a YAML/TOML/JSON config file and returns flat key→value
// pairs suitable for merging into the main viper instance.
//
// Fix C-5: the previous implementation returned v.AllSettings() which produces
// a *nested* map[string]interface{} (e.g. map["db"]["host"]).  When merged via
// v.Set("db", nestedMap) this clobbers the entire "db" namespace, silently
// discarding env-var overrides for individual sub-keys like "db.host".
//
// The correct approach is to let viper do the merge via MergeConfigMap, which
// understands the nested structure and merges key-by-key at every level.
type FileProvider struct {
	Path string
}

func (f *FileProvider) Name() string { return "file:" + f.Path }

func (f *FileProvider) UseDeepMerge() bool { return true }

func (f *FileProvider) Load() (map[string]interface{}, error) {
	v := viper.New()
	v.SetConfigFile(f.Path)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v.AllSettings(), nil
}
