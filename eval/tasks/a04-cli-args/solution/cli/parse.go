package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func Parse(args []string) (Config, error) {
	c := Config{Count: 1, Output: "-"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			c.Args = append(c.Args, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			c.Args = append(c.Args, a)
			continue
		}
		name, val, hasVal := a, "", false
		if j := strings.IndexByte(a, '='); j >= 0 {
			name, val, hasVal = a[:j], a[j+1:], true
		}
		need := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%w: flag %s needs a value", ErrUsage, name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "-v", "--verbose":
			if hasVal {
				return c, fmt.Errorf("%w: flag %s takes no value", ErrUsage, name)
			}
			c.Verbose = true
		case "-n", "--count":
			v, err := need()
			if err != nil {
				return c, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return c, fmt.Errorf("%w: flag %s: bad count %q", ErrUsage, name, v)
			}
			c.Count = n
		case "-o", "--output":
			v, err := need()
			if err != nil {
				return c, err
			}
			c.Output = v
		case "--tag":
			v, err := need()
			if err != nil {
				return c, err
			}
			c.Tags = append(c.Tags, v)
		default:
			if !strings.HasPrefix(name, "--") && !hasVal && strings.Trim(name[1:], "v") == "" {
				c.Verbose = true
				continue
			}
			return c, fmt.Errorf("%w: unknown flag %s", ErrUsage, name)
		}
	}
	return c, nil
}
