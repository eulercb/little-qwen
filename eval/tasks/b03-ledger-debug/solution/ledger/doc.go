// Package ledger replays an event log into account balances.
//
// # Specification
//
// Events are JSON objects, one per line (see testdata/events.jsonl):
//
//		{"id":"e123","seq":123,"ts":"2026-01-02T03:04:05Z","type":"deposit","account":"acc-07","amount":1250,"currency":"USD"}
//
//	  - amount is an integer number of minor units (cents) in the event's currency.
//	  - currency is "USD", "EUR" or "GBP". Balances are kept in USD cents. Non-USD
//	    amounts are converted with the fixed Rates table: usd = amount * rate,
//	    rounded to the nearest cent with ties to even (banker's rounding),
//	    per event, before it is applied.
//	  - type "deposit" adds the converted amount to account.
//	  - type "withdrawal" subtracts it from account. Balances may go negative.
//	  - type "transfer" subtracts from account and adds to the "to" account.
//	  - type "reversal" undoes the event named by "reverses" (an event id):
//	    the original event's effect is reverted exactly (same converted amount).
//	    A reversal of an unknown event, or a second reversal of the same event,
//	    is ignored.
//
// Delivery is at-least-once: the same event (same id) may appear more than
// once in the log; only the first occurrence counts.
//
// The log is not sorted. Events must be applied in timestamp order; events
// with equal timestamps are applied in seq order. A reversal may therefore
// appear in the file before the event it reverses, as long as its timestamp
// is later.
package ledger
