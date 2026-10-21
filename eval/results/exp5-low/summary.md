Harness: 13c005f2aeb0 — 8 runs over 8 tasks

| metric | mean ± sd |
|---|---|
| success | 0.88 ± 0.35 |
| score | 0.99 ± 0.03 |
| wall (min) | 18.6 ± 18.9 |
| turns | 16.2 ± 10.2 |
| prefill tokens | 20821 ± 17821 |
| re-prefill tokens | 71 ± 200 |
| completion tokens | 17960 ± 15985 |
| reasoning tokens | 11749 ± 12753 |
| tool calls / errors / repaired | 156 / 9 (5.8%) / 0 |

Failure codes: F10=1 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| a03-http-api | tierA | 1.00 | 1.00 | finished | 9 | 9.3 | 0 |
| a05-ini-bugfix | tierA | 1.00 | 1.00 | finished | 13 | 9.6 | 0 |
| a07-semver | tierA | 0.00 | 0.93 | finished | 14 | 6.2 | 0 |
| a09-csv-report | tierA | 1.00 | 1.00 | finished | 14 | 14.8 | 0 |
| a10-topk-words | tierA | 1.00 | 1.00 | wall_clock | 41 | 60.5 | 0 |
| a12-jsonpath | tierA | 1.00 | 1.00 | finished | 16 | 11.5 | 0 |
| h02-config-merge | heldout | 1.00 | 1.00 | finished | 11 | 5.0 | 0 |
| h04-mini-template | heldout | 1.00 | 1.00 | finished | 12 | 32.1 | 0 |
