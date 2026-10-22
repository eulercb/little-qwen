Harness: 6e880e0d3539 — 19 runs over 19 tasks

| metric | mean ± sd |
|---|---|
| success | 0.89 ± 0.32 |
| score | 0.99 ± 0.03 |
| wall (min) | 14.4 ± 12.6 |
| turns | 14.4 ± 9.6 |
| prefill tokens | 14843 ± 12260 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 10547 ± 11042 |
| reasoning tokens | 5440 ± 7068 |
| tool calls / errors / repaired | 365 / 24 (6.6%) / 1 |

Failure codes: F3=1 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| a01-lru-cache | tierA | 1.00 | 1.00 | finished | 7 | 7.1 | 0 |
| a02-fix-race | tierA | 1.00 | 1.00 | finished | 4 | 3.5 | 0 |
| a03-http-api | tierA | 1.00 | 1.00 | finished | 8 | 7.6 | 0 |
| a04-cli-args | tierA | 1.00 | 1.00 | finished | 25 | 21.4 | 0 |
| a05-ini-bugfix | tierA | 1.00 | 1.00 | finished | 7 | 11.5 | 0 |
| a06-rename-refactor | tierA | 1.00 | 1.00 | finished | 12 | 6.4 | 0 |
| a07-semver | tierA | 1.00 | 1.00 | finished | 11 | 9.7 | 0 |
| a08-token-bucket | tierA | 1.00 | 1.00 | finished | 5 | 7.2 | 0 |
| a09-csv-report | tierA | 1.00 | 1.00 | finished | 13 | 17.3 | 0 |
| a10-topk-words | tierA | 1.00 | 1.00 | finished | 27 | 42.9 | 0 |
| a11-bank-deadlock | tierA | 1.00 | 1.00 | finished | 8 | 4.5 | 0 |
| a12-jsonpath | tierA | 1.00 | 1.00 | finished | 18 | 21.7 | 0 |
| b01-expr-lang | tierB | 1.00 | 1.00 | finished | 36 | 46.7 | 0 |
| b02-tasker-feature | tierB | 1.00 | 1.00 | finished | 30 | 19.9 | 0 |
| b03-ledger-debug | tierB | 1.00 | 1.00 | finished | 8 | 4.7 | 0 |
| h01-paginate-bugs | heldout | 1.00 | 1.00 | finished | 9 | 3.2 | 0 |
| h02-config-merge | heldout | 0.00 | 0.92 | finished | 7 | 2.3 | 0 |
| h03-worker-pool | heldout | 1.00 | 1.00 | finished | 13 | 16.3 | 0 |
| h04-mini-template | heldout | 0.00 | 0.92 | finished | 26 | 19.9 | 0 |
