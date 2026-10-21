package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func NewHandler(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, 405, "method not allowed")
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			q := strings.ToLower(r.URL.Query().Get("q"))
			out := []Note{}
			for _, n := range s.List() {
				if q == "" || strings.Contains(strings.ToLower(n.Title), q) || strings.Contains(strings.ToLower(n.Body), q) {
					out = append(out, n)
				}
			}
			writeJSON(w, 200, out)
		case http.MethodPost:
			var in struct{ Title, Body string }
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeErr(w, 400, "bad json")
				return
			}
			if strings.TrimSpace(in.Title) == "" {
				writeErr(w, 400, "title required")
				return
			}
			writeJSON(w, 201, s.Create(in.Title, in.Body))
		default:
			writeErr(w, 405, "method not allowed")
		}
	})
	mux.HandleFunc("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeErr(w, 400, "bad id")
			return
		}
		switch r.Method {
		case http.MethodGet:
			n, ok := s.Get(id)
			if !ok {
				writeErr(w, 404, "not found")
				return
			}
			writeJSON(w, 200, n)
		case http.MethodDelete:
			if !s.Delete(id) {
				writeErr(w, 404, "not found")
				return
			}
			w.WriteHeader(204)
		default:
			writeErr(w, 405, "method not allowed")
		}
	})
	return mux
}
