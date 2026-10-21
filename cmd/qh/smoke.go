package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/repair"
	"github.com/eulercb/little-qwen/internal/tools"
)

type smokeCase struct {
	prompt string
	tool   string
	need   []string
}

var smokeCases = []smokeCase{
	{"Show me the contents of main.go.", "read_file", []string{"path"}},
	{"Read lines 40 to 80 of internal/server/handler.go.", "read_file", []string{"path"}},
	{"What's in go.mod?", "read_file", []string{"path"}},
	{"Open README.md and show it.", "read_file", []string{"path"}},
	{"Look at the first 20 lines of cmd/app/main.go.", "read_file", []string{"path"}},
	{"Create a file hello.txt containing exactly the text: hello world", "write_file", []string{"path", "content"}},
	{"Write a Go file util/strings.go in package util with a function Reverse(s string) string that reverses runes.", "write_file", []string{"path", "content"}},
	{"Create .gitignore with the lines bin/ and *.log", "write_file", []string{"path", "content"}},
	{"Make a new file notes/todo.md with a markdown checklist of three items: build, test, ship.", "write_file", []string{"path", "content"}},
	{"Create config.json containing {\"port\": 8080, \"debug\": true}.", "write_file", []string{"path", "content"}},
	{"In main.go, replace the line `port := 8080` with `port := 9090`.", "edit_file", []string{"path", "old_text", "new_text"}},
	{"In server.go change the string \"Hello, World\" to \"Hello, Qwen\".", "edit_file", []string{"path", "old_text", "new_text"}},
	{"In config.yaml, change `debug: false` to `debug: true`.", "edit_file", []string{"path", "old_text", "new_text"}},
	{"Rename the function call `oldName(x)` to `newName(x)` in lib/a.go (it appears once).", "edit_file", []string{"path", "old_text", "new_text"}},
	{"In README.md, replace the heading `# Project` with `# My Project`.", "edit_file", []string{"path", "old_text", "new_text"}},
	{"Run the Go tests for the whole module.", "run", []string{"command"}},
	{"Build the project with go build ./...", "run", []string{"command"}},
	{"Show the git log, last 5 commits, one line each.", "run", []string{"command"}},
	{"Check which Go version is installed.", "run", []string{"command"}},
	{"Run go vet on all packages.", "run", []string{"command"}},
	{"Show git status.", "run", []string{"command"}},
	{"Run only the test TestParse in ./parser with verbose output.", "run", []string{"command"}},
	{"Count the lines of all .go files with wc.", "run", []string{"command"}},
	{"Run `make lint`.", "run", []string{"command"}},
	{"Run the race detector on the tests in ./cache.", "run", []string{"command"}},
	{"Find every TODO comment in the codebase.", "grep", []string{"pattern"}},
	{"Where is the function ParseConfig defined?", "grep", []string{"pattern"}},
	{"Search the Go files for uses of `http.ListenAndServe`.", "grep", []string{"pattern"}},
	{"Find all places that import \"encoding/json\".", "grep", []string{"pattern"}},
	{"Which files mention the environment variable DATABASE_URL?", "grep", []string{"pattern"}},
	{"Search for the regex `func \\(s \\*Server\\)` in the internal directory.", "grep", []string{"pattern"}},
	{"Grep for 'panic(' in .go files only.", "grep", []string{"pattern"}},
	{"List the files in the project.", "list_dir", nil},
	{"Show the directory tree of internal/ three levels deep.", "list_dir", nil},
	{"What files are in the cmd directory?", "list_dir", nil},
	{"Show me what's in the testdata folder.", "list_dir", nil},
	{"Give me an overview of the repository layout.", "list_dir", nil},
	{"Mark the plan step '1. write parser' as done.", "update_plan", []string{"step", "status"}},
	{"Record in the plan that step '2. add tests' is now in progress (doing).", "update_plan", []string{"step", "status"}},
	{"Add a plan step '3. update docs' with status todo.", "update_plan", []string{"step", "status"}},
	{"Mark plan step '4. migrate db' as blocked with note 'needs schema'.", "update_plan", []string{"step", "status"}},
	{"Ask a sub-agent to scan the whole repository and report every place where errors are ignored with `_ =`. Do not do it yourself.", "delegate", []string{"task"}},
	{"Delegate this to a sub-agent: read all files under docs/ and summarise which configuration flags exist.", "delegate", []string{"task"}},
	{"Use a sub-agent to find which package implements the retry logic and report its exported API.", "delegate", []string{"task"}},
	{"The work is complete: you added the Reverse function and tests pass. Declare the task finished.", "finish", []string{"summary"}},
	{"Everything is done and verified with go test. Finish the task now with a summary.", "finish", []string{"summary"}},
	{"Read the file pkg/math/add.go.", "read_file", []string{"path"}},
	{"Delete the build cache with go clean -cache.", "run", []string{"command"}},
	{"Create an empty file .keep in the logs directory.", "write_file", []string{"path"}},
	{"Look for the word 'deprecated' in all markdown files.", "grep", []string{"pattern"}},
}

// cmdSmoke checks single-tool-call reliability (Phase 0 acceptance: >=95%).
func cmdSmoke(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ExitOnError)
	n := fs.Int("n", len(smokeCases), "number of cases")
	hdir := fs.String("harness", "harness/base", "harness dir")
	effort := fs.String("effort", "low", "reasoning effort")
	fs.Parse(args)
	h, err := loadHarness(*hdir)
	if err != nil {
		return err
	}
	c, err := llm.New()
	if err != nil {
		return err
	}
	c.Log = logger("")
	defs := tools.Definitions(nil)
	pass, structured, repaired := 0, 0, 0
	start := time.Now()
	for i, sc := range smokeCases {
		if i >= *n {
			break
		}
		msgs := []llm.Message{
			{Role: "system", Content: llm.Str(h.System)},
			{Role: "user", Content: llm.Str("# Task\n" + sc.prompt + "\n\nThis is a single-step tool test: respond with exactly one tool call.")},
		}
		resp, err := c.Chat(ctx, llm.Request{Messages: msgs, Tools: defs, MaxTokens: 4000, ReasoningEffort: *effort})
		if err != nil {
			fmt.Printf("%2d FAIL error %v\n", i, err)
			continue
		}
		calls := resp.Message.ToolCalls
		how := "structured"
		if len(calls) == 0 && repair.LooksLikeToolCall(resp.Message.Text()) {
			calls = repair.ParseContent(resp.Message.Text())
			how = "repaired"
		}
		verdict := "FAIL"
		detail := ""
		if len(calls) == 0 {
			detail = "no tool call: " + strings.Join(strings.Fields(resp.Message.Text()), " ")
		} else {
			tc := calls[0]
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &m); err != nil {
				detail = "bad json args"
			} else if tc.Function.Name != sc.tool {
				detail = fmt.Sprintf("wrong tool %s (want %s)", tc.Function.Name, sc.tool)
			} else {
				missing := []string{}
				for _, k := range sc.need {
					if _, ok := m[k]; !ok {
						missing = append(missing, k)
					}
				}
				if len(missing) > 0 {
					detail = "missing args " + strings.Join(missing, ",")
				} else {
					verdict = "PASS"
					pass++
					if how == "structured" {
						structured++
					} else {
						repaired++
					}
				}
			}
			detail = how + " " + tc.Function.Name + " " + trunc(tc.Function.Arguments, 100) + " " + detail
		}
		fmt.Printf("%2d %s %.1fs think=%d %s\n", i, verdict, float64(resp.LatencyMS)/1000, resp.Usage.ReasoningTokens, trunc(detail, 200))
	}
	total := min(*n, len(smokeCases))
	fmt.Printf("\nsmoke: %d/%d pass (%.0f%%), structured=%d repaired=%d, %.1f min\n", pass, total, 100*float64(pass)/float64(total), structured, repaired, time.Since(start).Minutes())
	return nil
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
