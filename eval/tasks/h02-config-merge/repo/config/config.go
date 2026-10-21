// Package config builds the application configuration.
package config

import (
	"fmt"
	"strconv"
)

// Config holds application settings.
type Config struct {
	Host    string
	Port    int
	Debug   bool
	Workers int
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	return Config{Host: "localhost", Port: 8080, Workers: 4}
}

// Load starts from Defaults and applies environment overrides from getenv:
// APP_HOST, APP_PORT (1-65535), APP_DEBUG (strconv.ParseBool), APP_WORKERS (>= 1).
func Load(getenv func(string) string) (Config, error) {
	c := Defaults()
	if v := getenv("APP_HOST"); v != "" {
		c.Host = v
	}
	if v := getenv("APP_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("APP_PORT: invalid port %q", v)
		}
		c.Port = p
	}
	if v := getenv("APP_DEBUG"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("APP_DEBUG: invalid bool %q", v)
		}
		c.Debug = b
	}
	if v := getenv("APP_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("APP_WORKERS: invalid count %q", v)
		}
		c.Workers = n
	}
	return c, nil
}
