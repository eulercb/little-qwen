package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHiddenNewFields(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_TIMEOUT": "1m30s", "APP_TAGS": " a, ,b ,c,"}))
	if err != nil || c.Timeout != 90*time.Second || !reflect.DeepEqual(c.Tags, []string{"a", "b", "c"}) {
		t.Fatalf("%+v %v", c, err)
	}
	d, _ := Load(env(nil))
	if d.Timeout != 30*time.Second || len(d.Tags) != 0 {
		t.Fatalf("defaults %+v", d)
	}
}

func TestHiddenAllErrors(t *testing.T) {
	c, err := Load(env(map[string]string{"APP_PORT": "0", "APP_DEBUG": "maybe", "APP_WORKERS": "-1", "APP_TIMEOUT": "soon", "APP_HOST": "h"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, v := range []string{"APP_PORT", "APP_DEBUG", "APP_WORKERS", "APP_TIMEOUT"} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error %q missing %s", err, v)
		}
	}
	if !reflect.DeepEqual(c, Config{}) {
		t.Fatalf("config not zero: %+v", c)
	}
	_, err = Load(env(map[string]string{"APP_PORT": "70000"}))
	if err == nil || !strings.Contains(err.Error(), "APP_PORT") || strings.Contains(err.Error(), "APP_DEBUG") {
		t.Fatalf("%v", err)
	}
}

func TestHiddenMerge(t *testing.T) {
	base := Config{Host: "a", Port: 1, Workers: 2, Timeout: time.Second, Tags: []string{"x"}}
	got := Merge(base, Config{Port: 9, Debug: true})
	want := Config{Host: "a", Port: 9, Debug: true, Workers: 2, Timeout: time.Second, Tags: []string{"x"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	got = Merge(base, Config{Tags: []string{}})
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("empty non-nil override must replace: %#v", got.Tags)
	}
	got = Merge(base, Config{Tags: []string{"y"}})
	got.Tags[0] = "mutated"
	if base.Tags[0] != "x" {
		t.Fatal("base modified")
	}
	ov := Config{Tags: []string{"o"}}
	got = Merge(base, ov)
	got.Tags[0] = "mutated"
	if ov.Tags[0] != "o" {
		t.Fatal("override aliased")
	}
	got = Merge(base, Config{})
	got.Tags[0] = "mutated"
	if base.Tags[0] != "x" {
		t.Fatal("base tags aliased")
	}
}
