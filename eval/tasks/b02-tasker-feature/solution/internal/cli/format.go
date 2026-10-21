package cli

import (
	"fmt"
	"strings"

	"example.com/tasker/internal/model"
)

func FormatTask(t model.Task) string {
	mark := " "
	if t.Done {
		mark = "x"
	}
	s := fmt.Sprintf("#%d [%s] P%d %s", t.ID, mark, t.Priority, t.Title)
	if t.Due != nil {
		s += " (due " + t.Due.Format("2006-01-02") + ")"
	}
	if len(t.Tags) > 0 {
		s += " [" + strings.Join(t.Tags, ",") + "]"
	}
	return s
}
