# Tool guidance
- read_file shows line numbers; they are not part of the file. Never copy the line-number prefix into old_text or file content.
- edit_file needs old_text to match exactly once, including indentation (Go uses tabs). If it fails, re-read the region and retry with the exact text.
- run output is capped; very long output is saved under .qh/spill/ and you can read_file it.
- Use grep to locate code instead of reading many whole files.
- delegate only for broad read-only investigations whose answer is short.
