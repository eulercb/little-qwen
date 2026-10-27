Harness: 3580e80c7ee2 — 3 runs over 3 tasks

| metric | mean ± sd |
|---|---|
| success | 0.67 ± 0.58 |
| score | 0.99 ± 0.02 |
| wall (min) | 40.5 ± 18.6 |
| turns | 31.7 ± 14.6 |
| prefill tokens | 46477 ± 19274 |
| re-prefill tokens | 0 ± 0 |
| completion tokens | 30168 ± 9532 |
| reasoning tokens | 14345 ± 5980 |
| tool calls / errors / repaired | 141 / 7 (5.0%) / 0 |

Failure codes: F4=3 F10=1 

| task | tier | success | score | status | turns | wall min | compactions |
|---|---|---|---|---|---|---|---|
| b01-expr-lang | tierB | 1.00 | 1.00 | finished | 30 | 29.3 | 0 |
| b05-port-cron | tierB | 1.00 | 1.00 | finished | 18 | 30.3 | 0 |
| b06-markdown | tierB | 0.00 | 0.96 | wall_clock | 47 | 62.0 | 0 |
