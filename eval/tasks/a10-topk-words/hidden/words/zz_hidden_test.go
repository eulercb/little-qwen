package words

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestHiddenBasicTies(t *testing.T) {
	got, err := TopK(strings.NewReader("b a c b a d"), 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []WordCount{{"a", 2}, {"b", 2}, {"c", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestHiddenApostrophes(t *testing.T) {
	got, _ := TopK(strings.NewReader("Don't stop! 'quoted' don't o'clock rock'n'roll it''s"), 10, nil)
	m := map[string]int{}
	for _, wc := range got {
		m[wc.Word] = wc.Count
	}
	if m["don't"] != 2 || m["quoted"] != 1 || m["o'clock"] != 1 || m["rock'n'roll"] != 1 {
		t.Fatalf("%v", m)
	}
	if _, ok := m["'quoted'"]; ok {
		t.Fatal("leading apostrophe kept")
	}
	if m["it"] != 1 || m["s"] != 1 {
		t.Fatalf("double apostrophe should split: %v", m)
	}
}

func TestHiddenUnicodeAndStop(t *testing.T) {
	got, _ := TopK(strings.NewReader("Ça va? ÇA VA, the The año 2024 año"), 10, []string{"THE"})
	want := []WordCount{{"año", 2}, {"va", 2}, {"ça", 2}, {"2024", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestHiddenKZero(t *testing.T) {
	got, err := TopK(strings.NewReader("a"), 0, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestHiddenLongLine(t *testing.T) {
	s := strings.Repeat("word ", 300000) + strings.Repeat("x", 200000)
	got, err := TopK(strings.NewReader(s), 1, nil)
	if err != nil || len(got) != 1 || got[0] != (WordCount{"word", 300000}) {
		t.Fatalf("%v %v", got, err)
	}
}

type errReader struct{ n int }

func (e *errReader) Read(p []byte) (int, error) {
	if e.n == 0 {
		e.n++
		return copy(p, "hello world "), nil
	}
	return 0, errors.New("boom")
}

func TestHiddenReadError(t *testing.T) {
	_, err := TopK(&errReader{}, 2, nil)
	if err == nil || err == io.EOF || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}
