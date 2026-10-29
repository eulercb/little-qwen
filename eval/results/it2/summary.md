Harness: a94d56e41335 — 7 runs over 7 tasks

| metric | mean ± sd |
|---|---|
| success | 0.57 ± 0.53 |
| score | 0.85 ± 0.37 |
| wall (min) | 53.8 ± 36.1 |
| turns | 44.6 ± 36.9 |
| prefill tokens | 64407 ± 54411 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 37896 ± 22629 |
| reasoning tokens | 17907 ± 8812 |
| tool calls / errors / repaired | 474 / 27 (5.7%) / 0 |

Failure codes: F4=9 F10=3 F3=2 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| b01-expr-lang | tierB | 1.00 | 1.00 | finished | 40 | 44.6 | 0 |
| b02-tasker-feature | tierB | 1.00 | 1.00 | finished | 21 | 18.9 | 0 |
| b03-ledger-debug | tierB | 1.00 | 1.00 | finished | 6 | 7.2 | 0 |
| b04-kv-server | tierB | 0.00 | 0.97 | wall_clock | 46 | 60.8 | 0 |
| b05-port-cron | tierB | 1.00 | 1.00 | wall_clock | 42 | 60.1 | 0 |
| b06-markdown | tierB | 0.00 | 0.96 | wall_clock | 35 | 67.7 | 0 |
| d01-caca-palavras | tierD | 0.00 | 0.00 | finished | 122 | 117.6 | 0 |
