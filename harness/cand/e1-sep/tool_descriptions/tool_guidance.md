# Tool guidance
- read_file prefixes each line with its number and `| ` (e.g. `   12| \tx := 1`). The prefix is not part of the file: everything after `| ` is the exact line content, including leading tabs.
- edit_file needs old_text to match exactly once, including indentation (Go uses tabs). If it fails, re-read the region and retry with the exact text.
- run output is capped; very long output is saved under .qh/spill/ and you can read_file it.
- Use grep to locate code instead of reading many whole files.
- delegate only for broad read-only investigations whose answer is short.
