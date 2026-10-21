package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHiddenHealth(t *testing.T) {
	rec := do(t, NewHandler(NewStore()), "GET", "/healthz", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestHiddenCreateAndGet(t *testing.T) {
	h := NewHandler(NewStore())
	rec := do(t, h, "POST", "/notes", `{"title":"Hello","body":"World"}`)
	if rec.Code != 201 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("create: %d %v", rec.Code, rec.Header())
	}
	var n Note
	json.Unmarshal(rec.Body.Bytes(), &n)
	if n.ID != 1 || n.Title != "Hello" || n.Body != "World" {
		t.Fatalf("note %+v", n)
	}
	rec = do(t, h, "GET", "/notes/1", "")
	if rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m["id"] != float64(1) || m["title"] != "Hello" {
		t.Fatalf("json shape %v", m)
	}
}

func TestHiddenValidation(t *testing.T) {
	h := NewHandler(NewStore())
	for _, body := range []string{`{"title":"   ","body":"x"}`, `{"body":"x"}`, `not json`} {
		rec := do(t, h, "POST", "/notes", body)
		if rec.Code != 400 {
			t.Fatalf("%s -> %d", body, rec.Code)
		}
		var e map[string]string
		if json.Unmarshal(rec.Body.Bytes(), &e) != nil || e["error"] == "" {
			t.Fatalf("error body %q", rec.Body.String())
		}
	}
}

func TestHiddenListEmptyIsArray(t *testing.T) {
	rec := do(t, NewHandler(NewStore()), "GET", "/notes", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestHiddenListFilter(t *testing.T) {
	s := NewStore()
	s.Create("Groceries", "milk and EGGS")
	s.Create("Work", "deploy")
	s.Create("Eggs recipe", "")
	h := NewHandler(s)
	rec := do(t, h, "GET", "/notes?q=eggs", "")
	var ns []Note
	json.Unmarshal(rec.Body.Bytes(), &ns)
	if len(ns) != 2 || ns[0].ID != 1 || ns[1].ID != 3 {
		t.Fatalf("filter: %+v", ns)
	}
}

func TestHiddenNotFoundAndBadID(t *testing.T) {
	h := NewHandler(NewStore())
	if c := do(t, h, "GET", "/notes/42", "").Code; c != 404 {
		t.Fatalf("missing get %d", c)
	}
	if c := do(t, h, "GET", "/notes/abc", "").Code; c != 400 {
		t.Fatalf("bad id %d", c)
	}
	if c := do(t, h, "DELETE", "/notes/7", "").Code; c != 404 {
		t.Fatalf("missing delete %d", c)
	}
}

func TestHiddenDelete(t *testing.T) {
	s := NewStore()
	s.Create("a", "")
	h := NewHandler(s)
	if c := do(t, h, "DELETE", "/notes/1", "").Code; c != 204 {
		t.Fatalf("delete %d", c)
	}
	if c := do(t, h, "GET", "/notes/1", "").Code; c != 404 {
		t.Fatalf("after delete %d", c)
	}
}

func TestHiddenMethodNotAllowed(t *testing.T) {
	h := NewHandler(NewStore())
	if c := do(t, h, "PUT", "/notes", "").Code; c != 405 {
		t.Fatalf("PUT /notes %d", c)
	}
	s := NewStore()
	s.Create("a", "")
	if c := do(t, NewHandler(s), "PATCH", "/notes/1", "").Code; c != 405 {
		t.Fatalf("PATCH %d", c)
	}
}
