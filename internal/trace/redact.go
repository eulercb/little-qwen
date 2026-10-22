package trace

import "regexp"

// Some benchmark tasks (secret sanitisation, leak recovery) contain fake
// credentials that end up in logs and transcripts. Redact common token
// formats before anything is written, so results can be committed and
// pushed (GitHub push protection rejects them).
var secretRe = regexp.MustCompile(`\b(hf_[A-Za-z0-9]{30,}|gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|sk-[A-Za-z0-9_-]{32,}|xox[abpr]-[A-Za-z0-9-]{20,}|AIza[0-9A-Za-z_-]{35})\b`)

// Redact replaces token-like substrings with a marker.
func Redact(s string) string {
	return secretRe.ReplaceAllStringFunc(s, func(m string) string {
		n := 4
		if len(m) < n {
			n = len(m)
		}
		return m[:n] + "…[REDACTED]"
	})
}
