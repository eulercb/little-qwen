# tasker

A small command-line task tracker that stores tasks in a JSON file.

```
tasker add [-p N] <title words...>     add a task (priority 1-5, default 3)
tasker list [--all] [--sort id|priority] list open tasks (or all)
tasker done <id>                        mark a task done
tasker rm <id>                          delete a task
tasker edit <id> [-p N] [--title T]     change priority or title
```

The data file is `$TASKER_FILE` (default `tasks.json` in the current directory).

## Layout
- `internal/model` — the Task type and validation.
- `internal/store` — loading/saving the JSON file (with a schema `version`).
- `internal/query` — filtering and sorting.
- `internal/cli` — argument parsing, commands and output formatting. `cli.Run` is the testable entry point.
- `cmd/tasker` — main.
