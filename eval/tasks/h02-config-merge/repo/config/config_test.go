package config

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil || c.Host != "localhost" || c.Port != 8080 || c.Workers != 4 {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestLoadOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_PORT": "9000", "APP_DEBUG": "true"}))
	if err != nil || c.Port != 9000 || !c.Debug {
		t.Fatalf("%+v %v", c, err)
	}
}
