package providers

import "errors"

type DBProvider struct{}

func (d *DBProvider) Name() string { return "db-remote" }

func (d *DBProvider) IsRequired() bool { return false }

func (d *DBProvider) Load() (map[string]interface{}, error) {
	// TODO: реализовать SELECT key, value FROM platform_config
	// и вернуть map[viperKey]value аналогично EnvProvider.
	return nil, errors.New("DBProvider: not implemented; register only after implementation is complete")
}
