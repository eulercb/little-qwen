package ledger

import (
	"testing"
	"time"
)

func zzhAt(s int) time.Time { return time.Date(2026, 5, 1, 0, 0, s, 0, time.UTC) }

func TestHiddenDuplicatesIgnored(t *testing.T) {
	ev := []Event{
		{ID: "a", Seq: 1, TS: zzhAt(1), Type: "deposit", Account: "x", Amount: 100, Currency: "USD"},
		{ID: "a", Seq: 1, TS: zzhAt(1), Type: "deposit", Account: "x", Amount: 100, Currency: "USD"},
	}
	got, err := Replay(ev)
	if err != nil || got["x"] != 100 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestHiddenReversalBeforeOriginalInFile(t *testing.T) {
	ev := []Event{
		{ID: "r", Seq: 9, TS: zzhAt(5), Type: "reversal", Reverses: "d", Currency: "USD"},
		{ID: "d", Seq: 2, TS: zzhAt(2), Type: "deposit", Account: "x", Amount: 700, Currency: "USD"},
		{ID: "k", Seq: 3, TS: zzhAt(3), Type: "deposit", Account: "x", Amount: 5, Currency: "USD"},
	}
	got, _ := Replay(ev)
	if got["x"] != 5 {
		t.Fatalf("x=%d", got["x"])
	}
}

func TestHiddenReversalEarlierThanOriginalIgnored(t *testing.T) {
	ev := []Event{
		{ID: "d", Seq: 2, TS: zzhAt(10), Type: "deposit", Account: "x", Amount: 700, Currency: "USD"},
		{ID: "r", Seq: 9, TS: zzhAt(5), Type: "reversal", Reverses: "d", Currency: "USD"},
	}
	got, _ := Replay(ev)
	if got["x"] != 700 {
		t.Fatalf("x=%d", got["x"])
	}
}

func TestHiddenEqualTimestampsUseSeq(t *testing.T) {
	ev := []Event{
		{ID: "r", Seq: 2, TS: zzhAt(1), Type: "reversal", Reverses: "d", Currency: "USD"},
		{ID: "d", Seq: 1, TS: zzhAt(1), Type: "deposit", Account: "x", Amount: 50, Currency: "USD"},
	}
	got, _ := Replay(ev)
	if got["x"] != 0 {
		t.Fatalf("x=%d", got["x"])
	}
}

func TestHiddenRounding(t *testing.T) {
	ev := []Event{
		{ID: "1", Seq: 1, TS: zzhAt(1), Type: "deposit", Account: "e", Amount: 7, Currency: "EUR"},    // 7.5775 -> 8
		{ID: "2", Seq: 2, TS: zzhAt(2), Type: "deposit", Account: "g", Amount: 3, Currency: "GBP"},    // 3.8025 -> 4
		{ID: "3", Seq: 3, TS: zzhAt(3), Type: "withdrawal", Account: "e", Amount: 1, Currency: "EUR"}, // 1.0825 -> 1
	}
	got, _ := Replay(ev)
	if got["e"] != 7 || got["g"] != 4 {
		t.Fatalf("%v", got)
	}
}

func TestHiddenTransferReversalTwice(t *testing.T) {
	ev := []Event{
		{ID: "t", Seq: 1, TS: zzhAt(1), Type: "transfer", Account: "a", To: "b", Amount: 1000, Currency: "EUR"},
		{ID: "r1", Seq: 2, TS: zzhAt(2), Type: "reversal", Reverses: "t"},
		{ID: "r2", Seq: 3, TS: zzhAt(3), Type: "reversal", Reverses: "t"},
		{ID: "r3", Seq: 4, TS: zzhAt(4), Type: "reversal", Reverses: "nope"},
	}
	got, err := Replay(ev)
	if err != nil || got["a"] != 0 || got["b"] != 0 {
		t.Fatalf("%v %v", got, err)
	}
}
