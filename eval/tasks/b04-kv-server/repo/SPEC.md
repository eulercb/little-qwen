# kv — specification

Module `example.com/kv`. Standard library only. Three packages.

## Package `store` (directory `store/`)

```go
type Clock func() time.Time

func New(clock Clock) *Store          // nil clock → time.Now
func (s *Store) Set(key, value string, ttl time.Duration)   // ttl <= 0 means no expiry; Set always replaces value AND ttl
func (s *Store) Get(key string) (string, bool)
func (s *Store) Del(keys ...string) int                      // number of keys that existed and were removed
func (s *Store) Exists(key string) bool
func (s *Store) Expire(key string, ttl time.Duration) bool   // false if key missing; ttl <= 0 deletes the key (returns true)
func (s *Store) Persist(key string) bool                     // remove expiry; true only if key existed AND had an expiry
func (s *Store) TTL(key string) time.Duration                // -2 (as time.Duration) if missing, -1 if no expiry, else remaining (>0)
func (s *Store) Incr(key string, by int64) (int64, error)    // missing key counts as 0; keeps existing ttl; ErrNotInteger if value is not a base-10 int64
func (s *Store) Keys(pattern string) []string                // glob match, sorted ascending
func (s *Store) Len() int                                    // number of live keys
func (s *Store) Save(w io.Writer) error                      // snapshot, see below
func (s *Store) Load(r io.Reader) error                      // replaces all contents
var ErrNotInteger = errors.New("value is not an integer")
```

- A key is expired when `now >= deadline`. Expired keys behave exactly like missing keys for every method (including `Len`, `Keys`, `Save`).
- Safe for concurrent use.
- Glob patterns for `Keys`: `*` any sequence (including empty), `?` exactly one character, `[abc]` one of the set, `[a-z]` a range, `[^a]` / `[!a]` negation, `\x` escapes `x`. The whole key must match. (`path.Match` is NOT suitable because `*` must also match `/`.)
- Snapshot format (`Save`): JSON object `{"version":1,"entries":[{"k":"<key>","v":"<value>","exp":<unix-milliseconds or 0>}, ...]}` with entries sorted by key. `Load` rejects other versions with an error and drops entries that are already expired at load time.

## Package `proto` (directory `proto/`)

A line protocol. `func Exec(s *store.Store, line string) string` parses one request line and returns the full response (including the trailing `\n`).

**Request tokenising:** tokens are separated by one or more spaces/tabs. A token may be double-quoted: `"hello world"`; inside quotes `\"`, `\\` and `\n` are escapes. An unterminated quote is a protocol error. Command names are case-insensitive. An empty line returns `-ERR empty command\n`.

**Responses:**
- simple: `+OK\n`
- string: `$<value>\n` — the value is written with the same quoting rules: if it contains a space, tab, `"`, `\`, newline, or is empty, it is written quoted with escapes, e.g. `$"a b"\n`, `$""\n`.
- nil: `$nil\n`
- integer: `:<n>\n`
- array: `*<count>\n` followed by one `$<value>\n` line per element (same quoting).
- error: `-ERR <message>\n`

**Commands** (`ttl` arguments are integer seconds):

| command | response |
|---|---|
| `PING` | `+PONG\n` |
| `SET key value` / `SET key value EX seconds` | `+OK\n` (EX must be a positive integer) |
| `GET key` | string or nil |
| `DEL key [key ...]` | integer |
| `EXISTS key` | `:1`/`:0` |
| `EXPIRE key seconds` | `:1` if set, `:0` if key missing |
| `PERSIST key` | `:1`/`:0` |
| `TTL key` | integer seconds remaining rounded UP (e.g. 1.2s → 2), `-1` no expiry, `-2` missing |
| `INCR key` / `INCRBY key n` | integer, or `-ERR value is not an integer\n` |
| `KEYS pattern` | array |
| `DBSIZE` | integer |

Errors: unknown command → `-ERR unknown command 'NAME'\n` (NAME as sent); wrong number of arguments → `-ERR wrong number of arguments for 'name'\n` (lower-case name); a non-integer where an integer is required → `-ERR value is not an integer\n`; syntax problems (bad quoting, bad SET option) → `-ERR syntax error\n`.

## Package `server` (directory `server/`)

```go
func Serve(ctx context.Context, l net.Listener, s *store.Store) error
```
Accepts TCP connections until `ctx` is cancelled (then closes the listener, closes open connections, waits for connection goroutines to finish and returns `nil`). Each connection reads request lines (terminated by `\n`, optional `\r` stripped) and writes `proto.Exec` responses in order. The command `QUIT` replies `+BYE\n` and closes that connection. Lines longer than 64 KiB get `-ERR line too long\n` and the connection is closed. Each client is served concurrently.
