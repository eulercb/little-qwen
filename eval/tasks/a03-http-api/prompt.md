Package `api` has an in-memory notes service with a `Store` (in `api/store.go`). Add an HTTP API in a new file `api/handler.go`:

`func NewHandler(s *Store) http.Handler` serving:

| Method & path | Behaviour |
|---|---|
| `GET /healthz` | 200, body `ok` (text/plain) |
| `POST /notes` | JSON body `{"title": "...", "body": "..."}`. `title` is required (non-empty after trimming spaces) → else 400. Creates a note and returns 201 with the created note as JSON. |
| `GET /notes` | 200, JSON array of all notes ordered by id ascending (empty array `[]`, not `null`, when none). Optional query `?q=word` filters to notes whose title or body contains `word` (case-insensitive). |
| `GET /notes/{id}` | 200 with the note as JSON, 404 if missing, 400 if id is not an integer. |
| `DELETE /notes/{id}` | 204 on success, 404 if missing. |

Any other method on these paths returns 405. JSON responses must set `Content-Type: application/json`. Error responses are JSON `{"error": "<message>"}`.
Note JSON shape: `{"id": 1, "title": "...", "body": "..."}`. Use only the standard library (Go 1.24 `http.ServeMux` patterns are allowed). Write tests with `net/http/httptest`.
