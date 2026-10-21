package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Rates converts one unit of a currency to USD.
var Rates = map[string]float64{"USD": 1, "EUR": 1.0825, "GBP": 1.2675}

// Event is one log entry.
type Event struct {
	ID       string    `json:"id"`
	Seq      int64     `json:"seq"`
	TS       time.Time `json:"ts"`
	Type     string    `json:"type"`
	Account  string    `json:"account"`
	To       string    `json:"to,omitempty"`
	Amount   int64     `json:"amount"`
	Currency string    `json:"currency"`
	Reverses string    `json:"reverses,omitempty"`
}

// ReadEvents parses a JSONL event log.
func ReadEvents(r io.Reader) ([]Event, error) {
	var out []Event
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		if len(sc.Bytes()) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// toUSD converts an amount in minor units to USD cents.
func toUSD(amount int64, currency string) (int64, error) {
	rate, ok := Rates[currency]
	if !ok {
		return 0, fmt.Errorf("unknown currency %q", currency)
	}
	return int64(float64(amount) * rate), nil
}

// Replay applies the events and returns balances in USD cents.
func Replay(events []Event) (map[string]int64, error) {
	bal := map[string]int64{}
	applied := map[string]Event{}
	reversed := map[string]bool{}
	for _, e := range events {
		switch e.Type {
		case "deposit", "withdrawal", "transfer":
			usd, err := toUSD(e.Amount, e.Currency)
			if err != nil {
				return nil, fmt.Errorf("event %s: %w", e.ID, err)
			}
			apply(bal, e, usd, 1)
			applied[e.ID] = e
		case "reversal":
			orig, ok := applied[e.Reverses]
			if !ok || reversed[e.Reverses] {
				continue
			}
			usd, err := toUSD(orig.Amount, orig.Currency)
			if err != nil {
				return nil, err
			}
			apply(bal, orig, usd, -1)
			reversed[e.Reverses] = true
		default:
			return nil, fmt.Errorf("event %s: unknown type %q", e.ID, e.Type)
		}
	}
	return bal, nil
}

func apply(bal map[string]int64, e Event, usd int64, sign int64) {
	switch e.Type {
	case "deposit":
		bal[e.Account] += sign * usd
	case "withdrawal":
		bal[e.Account] -= sign * usd
	case "transfer":
		bal[e.Account] -= sign * usd
		bal[e.To] += sign * usd
	}
}
