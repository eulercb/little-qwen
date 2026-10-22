package trace

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	// Assembled at runtime so the source itself does not look like a credential.
	in := "token " + "hf" + "_" + strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1) + "ABCDEFGH and " + "AK" + "IA" + "ABCDEFGHIJKLMNOP ok"
	out := Redact(in)
	if strings.Contains(out, "abcdefghijklmnopqrstuvwxyz") || strings.Contains(out, "ABCDEFGHIJKLMNOP") || !strings.Contains(out, "ok") {
		t.Fatal(out)
	}
}
