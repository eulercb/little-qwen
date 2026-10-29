Harness: a94d56e41335 — 6 runs over 6 tasks

| metric | mean ± sd |
|---|---|
| success | 0.67 ± 0.52 |
| score | 0.99 ± 0.02 |
| wall (min) | 43.2 ± 24.8 |
| turns | 31.7 ± 15.3 |
| prefill tokens | 44986 ± 19608 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 31004 ± 14681 |
| reasoning tokens | 15814 ± 7510 |
| tool calls / errors / repaired | 278 / 23 (8.3%) / 0 |

Failure codes: F4=6 F10=3 F3=2 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| b01-expr-lang | tierB | 1.00 | 1.00 | finished | 40 | 44.6 | 0 |
| b02-tasker-feature | tierB | 1.00 | 1.00 | finished | 21 | 18.9 | 0 |
| b03-ledger-debug | tierB | 1.00 | 1.00 | finished | 6 | 7.2 | 0 |
| b04-kv-server | tierB | 0.00 | 0.97 | wall_clock | 46 | 60.8 | 0 |
| b05-port-cron | tierB | 1.00 | 1.00 | wall_clock | 42 | 60.1 | 0 |
| b06-markdown | tierB | 0.00 | 0.96 | wall_clock | 35 | 67.7 | 0 |
