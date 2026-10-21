// Package cli parses the command line of the frob tool.
package cli

import "errors"

// ErrUsage is wrapped by every parse error.
var ErrUsage = errors.New("usage error")

// Config is the parsed command line.
type Config struct {
	Verbose bool
	Count   int
	Output  string
	Tags    []string
	Args    []string
}
