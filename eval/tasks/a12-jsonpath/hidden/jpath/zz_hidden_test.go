package jpath

import (
	"encoding/json"
	"reflect"
	"testing"
)

const zzhStore = `{"store":{"book":[{"title":"A","price":8,"tags":["x"]},{"title":"B","price":12},{"title":"C","price":5,"meta":{"price":1}}],"bike":{"price":100,"color":"red"}},"odd key":{"q\"k":7}}`

func TestHiddenTable(t *testing.T) {
	d := zzhDoc(t, zzhStore)
	cases := []struct {
		path string
		want []any
	}{
		{"$", []any{d}},
		{"$.store.book[0].title", []any{"A"}},
		{"$.store.book[-1].title", []any{"C"}},
		{"$.store.book[5]", []any{}},
		{"$.store.book[*].title", []any{"A", "B", "C"}},
		{"$.store.bike.*", []any{"red", 100.0}},
		{"$[\"odd key\"][\"q\\\"k\"]", []any{7.0}},
		{"$..price", []any{100.0, 8.0, 12.0, 5.0, 1.0}},
		{"$.store.book[*].meta.price", []any{1.0}},
		{"$.nope.deeper", []any{}},
		{"$.store.book.title", []any{}},
		{"$.store[0]", []any{}},
	}
	for _, c := range cases {
		got, err := Query(d, c.path)
		if err != nil {
			t.Errorf("%s: %v", c.path, err)
			continue
		}
		if got == nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %#v want %#v", c.path, got, c.want)
		}
	}
}

func TestHiddenSyntaxErrors(t *testing.T) {
	d := zzhDoc(t, zzhStore)
	for _, p := range []string{"", "a.b", "$.", "$[", "$[1", `$["x]`, "$.1abc", "$[abc]", "$..", "$ .a", "$.a[]"} {
		got, err := Query(d, p)
		if err == nil || got != nil {
			t.Errorf("%q: expected error, got %v %v", p, got, err)
		}
	}
}

func zzhDoc(t *testing.T, s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
