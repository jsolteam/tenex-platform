package config

import "errors"

var (
	ErrNotFound = errors.New("config: key not found")

	ErrEmptyKey = errors.New("config: key is required")
	
	ErrEmptyValue = errors.New("config: value is required")
)
