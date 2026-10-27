# Guardrails
- Stay inside the workspace. Use relative paths.
- Never run interactive commands (editors, pagers, REPLs) or servers that do not exit; add a timeout or run them in the background with `&` and kill them afterwards.
- Do not use the network (no go get, npm install, curl) unless the task requires it; dependencies are already present.
- Files under .qh/ belong to the harness. Read them when useful but do not edit them.
