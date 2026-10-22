Harness: 96f9f10eaa38 — 8 runs over 8 tasks

| metric | mean ± sd |
|---|---|
| success | 0.75 ± 0.46 |
| score | 0.92 ± 0.18 |
| wall (min) | 12.0 ± 12.2 |
| turns | 28.8 ± 32.0 |
| prefill tokens | 17927 ± 18772 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 12910 ± 12893 |
| reasoning tokens | 3414 ± 4191 |
| tool calls / errors / repaired | 253 / 14 (5.5%) / 0 |

Failure codes: F3=32 F10=2 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| a03-http-api | tierA | 1.00 | 1.00 | finished | 9 | 2.8 | 0 |
| a05-ini-bugfix | tierA | 1.00 | 1.00 | finished | 10 | 5.6 | 0 |
| a07-semver | tierA | 1.00 | 1.00 | finished | 7 | 3.9 | 0 |
| a09-csv-report | tierA | 1.00 | 1.00 | finished | 12 | 6.5 | 0 |
| a10-topk-words | tierA | 1.00 | 1.00 | finished | 23 | 13.3 | 0 |
| a12-jsonpath | tierA | 0.00 | 0.50 | max_turns | 80 | 27.0 | 0 |
| h02-config-merge | heldout | 1.00 | 1.00 | finished | 9 | 2.3 | 0 |
| h04-mini-template | heldout | 0.00 | 0.86 | max_turns | 80 | 34.5 | 0 |
