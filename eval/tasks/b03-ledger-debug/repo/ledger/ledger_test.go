package ledger

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

func TestReplayMatchesExpected(t *testing.T) {
	f, err := os.Open("../testdata/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := ReadEvents(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]int64
	data, _ := os.ReadFile("../testdata/expected_balances.json")
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	var accts []string
	for a := range want {
		accts = append(accts, a)
	}
	sort.Strings(accts)
	bad := 0
	for _, a := range accts {
		if got[a] != want[a] {
			bad++
			t.Errorf("account %s: got %d want %d (diff %d)", a, got[a], want[a], got[a]-want[a])
		}
	}
	if bad > 0 {
		t.Logf("%d of %d accounts wrong; replayed %d events", bad, len(accts), len(events))
	}
}

func TestSimpleDeposit(t *testing.T) {
	got, err := Replay([]Event{{ID: "a", Seq: 1, Type: "deposit", Account: "x", Amount: 500, Currency: "USD"}})
	if err != nil || got["x"] != 500 {
		t.Fatal(got, err)
	}
}
