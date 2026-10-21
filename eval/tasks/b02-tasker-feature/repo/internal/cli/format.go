package cli

import (
	"fmt"

	"example.com/tasker/internal/model"
)

// FormatTask renders one list line: "#<id> [x| ] P<priority> <title>".
func FormatTask(t model.Task) string {
	mark := " "
	if t.Done {
		mark = "x"
	}
	return fmt.Sprintf("#%d [%s] P%d %s", t.ID, mark, t.Priority, t.Title)
}
