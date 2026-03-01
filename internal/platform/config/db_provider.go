package config

type DBProvider struct{}

func (d *DBProvider) Name() string {
	return "db"
}

func (d *DBProvider) Load() (map[string]interface{}, error) {
	// TODO: implement load from postgres
	return map[string]interface{}{}, nil
}
