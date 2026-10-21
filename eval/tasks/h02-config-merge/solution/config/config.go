package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host    string
	Port    int
	Debug   bool
	Workers int
	Timeout time.Duration
	Tags    []string
}

func Defaults() Config {
	return Config{Host: "localhost", Port: 8080, Workers: 4, Timeout: 30 * time.Second}
}

func Load(getenv func(string) string) (Config, error) {
	c := Defaults()
	var errs []error
	if v := getenv("APP_HOST"); v != "" {
		c.Host = v
	}
	if v := getenv("APP_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("APP_PORT: invalid port %q", v))
		} else {
			c.Port = p
		}
	}
	if v := getenv("APP_DEBUG"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("APP_DEBUG: invalid bool %q", v))
		} else {
			c.Debug = b
		}
	}
	if v := getenv("APP_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Errorf("APP_WORKERS: invalid count %q", v))
		} else {
			c.Workers = n
		}
	}
	if v := getenv("APP_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("APP_TIMEOUT: invalid duration %q", v))
		} else {
			c.Timeout = d
		}
	}
	if v := getenv("APP_TAGS"); v != "" {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				c.Tags = append(c.Tags, t)
			}
		}
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

func Merge(base, o Config) Config {
	r := base
	r.Tags = append([]string(nil), base.Tags...)
	if base.Tags == nil {
		r.Tags = nil
	}
	if o.Host != "" {
		r.Host = o.Host
	}
	if o.Port != 0 {
		r.Port = o.Port
	}
	if o.Debug {
		r.Debug = true
	}
	if o.Workers != 0 {
		r.Workers = o.Workers
	}
	if o.Timeout != 0 {
		r.Timeout = o.Timeout
	}
	if o.Tags != nil {
		r.Tags = append([]string{}, o.Tags...)
	}
	return r
}
