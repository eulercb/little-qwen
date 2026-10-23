Harness: a01df662dc3a — 6 runs over 6 tasks

| metric | mean ± sd |
|---|---|
| success | 0.67 ± 0.52 |
| score | 0.83 ± 0.40 |
| wall (min) | 41.4 ± 19.7 |
| turns | 30.2 ± 23.4 |
| prefill tokens | 42003 ± 20954 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 28871 ± 17602 |
| reasoning tokens | 11871 ± 10479 |
| tool calls / errors / repaired | 235 / 26 (11.1%) / 0 |

Failure codes: F4=7 F10=2 F3=2 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| a10-topk-words | tierA | 1.00 | 1.00 | finished | 22 | 33.4 | 0 |
| b01-expr-lang | tierB | 0.00 | 0.95 | wall_clock | 42 | 60.3 | 0 |
| b04-kv-server | tierB | 1.00 | 1.00 | finished | 70 | 51.6 | 0 |
| b05-port-cron | tierB | 1.00 | 1.00 | finished | 30 | 33.0 | 0 |
| b06-markdown | tierB | 0.00 | 0.00 | wall_clock | 11 | 60.0 | 0 |
| h03-worker-pool | heldout | 1.00 | 1.00 | finished | 6 | 9.8 | 0 |
