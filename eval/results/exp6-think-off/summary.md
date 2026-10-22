Harness: 6ef836d18c55 — 8 runs over 8 tasks

| metric | mean ± sd |
|---|---|
| success | 0.50 ± 0.53 |
| score | 0.65 ± 0.44 |
| wall (min) | 10.3 ± 10.1 |
| turns | 46.0 ± 36.5 |
| prefill tokens | 16939 ± 12194 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 8605 ± 5935 |
| reasoning tokens | 0 ± 0 |
| tool calls / errors / repaired | 374 / 37 (9.9%) / 0 |

Failure codes: F3=53 F10=4 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| a03-http-api | tierA | 1.00 | 1.00 | finished | 10 | 2.5 | 0 |
| a05-ini-bugfix | tierA | 1.00 | 1.00 | finished | 10 | 1.4 | 0 |
| a07-semver | tierA | 0.00 | 0.80 | max_turns | 80 | 13.5 | 0 |
| a09-csv-report | tierA | 1.00 | 1.00 | finished | 9 | 3.1 | 0 |
| a10-topk-words | tierA | 0.00 | 0.00 | max_turns | 80 | 31.3 | 0 |
| a12-jsonpath | tierA | 0.00 | 0.33 | max_turns | 80 | 12.4 | 0 |
| h02-config-merge | heldout | 1.00 | 1.00 | finished | 19 | 3.2 | 0 |
| h04-mini-template | heldout | 0.00 | 0.11 | max_turns | 80 | 14.6 | 0 |
