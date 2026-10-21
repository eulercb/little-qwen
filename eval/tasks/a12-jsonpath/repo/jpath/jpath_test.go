package jpath

import (
	"encoding/json"
	"reflect"
	"testing"
)

func doc(t *testing.T, s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMember(t *testing.T) {
	d := doc(t, `{"a":{"b":[1,2,3]}}`)
	got, err := Query(d, "$.a.b[1]")
	if err != nil || !reflect.DeepEqual(got, []any{2.0}) {
		t.Fatalf("%v %v", got, err)
	}
}
