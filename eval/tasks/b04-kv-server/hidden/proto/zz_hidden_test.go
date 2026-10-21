package proto

import (
	"testing"
	"time"

	"example.com/kv/store"
)

func TestHiddenExecTable(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := store.New(func() time.Time { return now })
	steps := []struct{ in, out string }{
		{"PING", "+PONG\n"},
		{"ping", "+PONG\n"},
		{"", "-ERR empty command\n"},
		{"   ", "-ERR empty command\n"},
		{"GET a", "$nil\n"},
		{"SET a hello", "+OK\n"},
		{"get a", "$hello\n"},
		{`SET b "two words"`, "+OK\n"},
		{"GET b", "$\"two words\"\n"},
		{`SET c "q\"x\\y\nz"`, "+OK\n"},
		{"GET c", "$\"q\\\"x\\\\y\\nz\"\n"},
		{`SET e ""`, "+OK\n"},
		{"GET e", "$\"\"\n"},
		{`SET bad "unterminated`, "-ERR syntax error\n"},
		{"SET t v EX 10", "+OK\n"},
		{"TTL t", ":10\n"},
		{"TTL a", ":-1\n"},
		{"TTL zz", ":-2\n"},
		{"SET t v EX 0", "-ERR syntax error\n"},
		{"SET t v EX abc", "-ERR value is not an integer\n"},
		{"SET t v PX 10", "-ERR syntax error\n"},
		{"SET onlykey", "-ERR wrong number of arguments for 'set'\n"},
		{"DEL a b zz", ":2\n"},
		{"DEL", "-ERR wrong number of arguments for 'del'\n"},
		{"EXISTS a", ":0\n"},
		{"EXISTS c", ":1\n"},
		{"INCR n", ":1\n"},
		{"INCRBY n 41", ":42\n"},
		{"INCRBY n x", "-ERR value is not an integer\n"},
		{"INCR c", "-ERR value is not an integer\n"},
		{"EXPIRE n 5", ":1\n"},
		{"EXPIRE missing 5", ":0\n"},
		{"EXPIRE n soon", "-ERR value is not an integer\n"},
		{"PERSIST n", ":1\n"},
		{"PERSIST n", ":0\n"},
		{"KEYS n*", "*1\n$n\n"},
		{"DBSIZE", ":4\n"},
		{"FLY me", "-ERR unknown command 'FLY'\n"},
		{"GeT", "-ERR wrong number of arguments for 'get'\n"},
	}
	for _, st := range steps {
		if got := Exec(s, st.in); got != st.out {
			t.Errorf("Exec(%q) = %q want %q", st.in, got, st.out)
		}
	}
}

func TestHiddenTTLRoundsUp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := store.New(func() time.Time { return now })
	Exec(s, "SET k v EX 3")
	now = now.Add(1800 * time.Millisecond)
	if got := Exec(s, "TTL k"); got != ":2\n" {
		t.Fatalf("got %q", got)
	}
}

func TestHiddenKeysArrayQuoting(t *testing.T) {
	s := store.New(nil)
	Exec(s, `SET "a b" 1`)
	Exec(s, "SET ab 2")
	if got := Exec(s, "KEYS a*"); got != "*2\n$\"a b\"\n$ab\n" {
		t.Fatalf("%q", got)
	}
	if got := Exec(s, "KEYS zz*"); got != "*0\n" {
		t.Fatalf("%q", got)
	}
}
