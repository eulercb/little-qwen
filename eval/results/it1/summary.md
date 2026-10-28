Harness: a01327c09982 — 10 runs over 10 tasks

| metric | mean ± sd |
|---|---|
| success | 0.80 ± 0.42 |
| score | 0.89 ± 0.31 |
| wall (min) | 27.5 ± 23.4 |
| turns | 23.8 ± 17.1 |
| prefill tokens | 28716 ± 21053 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 18887 ± 14910 |
| reasoning tokens | 8378 ± 7309 |
| tool calls / errors / repaired | 312 / 23 (7.4%) / 0 |

Failure codes: F4=7 F10=2 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| b01-expr-lang | tierB | 1.00 | 1.00 | finished | 25 | 34.9 | 0 |
| b02-tasker-feature | tierB | 1.00 | 1.00 | finished | 27 | 18.3 | 0 |
| b03-ledger-debug | tierB | 1.00 | 1.00 | finished | 6 | 4.6 | 0 |
| b04-kv-server | tierB | 0.00 | 0.91 | wall_clock | 61 | 60.7 | 0 |
| b05-port-cron | tierB | 1.00 | 1.00 | finished | 43 | 53.4 | 0 |
| b06-markdown | tierB | 0.00 | 0.00 | wall_clock | 23 | 60.8 | 0 |
| h01-paginate-bugs | heldout | 1.00 | 1.00 | finished | 6 | 2.2 | 0 |
| h02-config-merge | heldout | 1.00 | 1.00 | finished | 12 | 4.1 | 0 |
| h03-worker-pool | heldout | 1.00 | 1.00 | finished | 17 | 19.1 | 0 |
| h04-mini-template | heldout | 1.00 | 1.00 | finished | 18 | 17.0 | 0 |
