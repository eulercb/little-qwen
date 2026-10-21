In package `words` (file `words/words.go`) implement:

`func TopK(r io.Reader, k int, stop []string) ([]WordCount, error)` with `type WordCount struct { Word string; Count int }`.

- A word is a maximal run of Unicode letters or digits, optionally containing internal apostrophes between letters (`don't`, `o'clock`); leading/trailing apostrophes are not part of the word. Everything else separates words.
- Words are case-folded with `strings.ToLower`.
- Words in `stop` (compare case-insensitively) are ignored.
- Return the k most frequent words, by count descending, ties broken by the word ascending (byte order). If there are fewer than k distinct words, return all. k <= 0 returns an empty (non-nil) slice.
- Input may be large (hundreds of MB): stream it, don't read it all into memory, and do not fail on very long lines.
- Return any read error from `r`.
