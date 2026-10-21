package cli

import "testing"

func TestDefaults(t *testing.T) {
	c, err := Parse(nil)
	if err != nil || c.Count != 1 || c.Output != "-" || c.Verbose {
		t.Fatalf("%+v %v", c, err)
	}
}
