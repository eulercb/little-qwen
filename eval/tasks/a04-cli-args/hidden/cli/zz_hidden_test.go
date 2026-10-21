package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestHiddenForms(t *testing.T) {
	cases := []struct {
		in   []string
		want Config
	}{
		{[]string{"-n", "3", "file"}, Config{Count: 3, Output: "-", Args: []string{"file"}}},
		{[]string{"-n=4"}, Config{Count: 4, Output: "-"}},
		{[]string{"--count", "0"}, Config{Count: 0, Output: "-"}},
		{[]string{"--count=7", "-v"}, Config{Count: 7, Output: "-", Verbose: true}},
		{[]string{"-o", "out.txt", "a", "b"}, Config{Count: 1, Output: "out.txt", Args: []string{"a", "b"}}},
		{[]string{"--output=x.json"}, Config{Count: 1, Output: "x.json"}},
		{[]string{"--tag", "a", "--tag=b", "x"}, Config{Count: 1, Output: "-", Tags: []string{"a", "b"}, Args: []string{"x"}}},
		{[]string{"-vv", "--verbose"}, Config{Count: 1, Output: "-", Verbose: true}},
		{[]string{"a", "--", "-n", "--weird"}, Config{Count: 1, Output: "-", Args: []string{"a", "-n", "--weird"}}},
		{[]string{"-", "-v"}, Config{Count: 1, Output: "-", Verbose: true, Args: []string{"-"}}},
		{[]string{"-o", "-"}, Config{Count: 1, Output: "-"}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("%v: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: got %+v want %+v", c.in, got, c.want)
		}
	}
}

func TestHiddenErrors(t *testing.T) {
	cases := []struct {
		in   []string
		flag string
	}{
		{[]string{"-n"}, "-n"},
		{[]string{"--count", "abc"}, "--count"},
		{[]string{"-n", "-2"}, "-n"},
		{[]string{"--bogus"}, "--bogus"},
		{[]string{"-x"}, "-x"},
		{[]string{"--output"}, "--output"},
		{[]string{"-vx"}, "-vx"},
	}
	for _, c := range cases {
		_, err := Parse(c.in)
		if err == nil {
			t.Errorf("%v: expected error", c.in)
			continue
		}
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%v: error does not wrap ErrUsage: %v", c.in, err)
		}
		if !strings.Contains(err.Error(), c.flag) {
			t.Errorf("%v: error %q does not mention %s", c.in, err, c.flag)
		}
	}
}

func TestHiddenNilSlices(t *testing.T) {
	c, _ := Parse([]string{"-v"})
	if c.Tags != nil || c.Args != nil {
		t.Fatalf("expected nil slices: %#v", c)
	}
}
