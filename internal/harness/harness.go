// Package harness loads and validates a Harness²-shaped component tree:
//
//	systemprompt.md, guardrails.md, LongTermMEMORY.md,
//	tool_descriptions/tool_guidance.md      (always-on text)
//	skills/<name>/SKILL.md (+ files)        (loaded on demand via read_file)
//	scripts/<name>                          (run via the run tool)
//	sub_agents/<name>/AGENT.md              (fresh-context delegates)
//	config.json                             (Go-side hook knobs)
//
// The always-on text becomes the system message. It is identical for every
// task run with the same harness, so the server's prefix cache is shared
// across tasks and sub-agents.
package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MaxAlwaysOnChars is Harness²'s cap on always-on text.
const MaxAlwaysOnChars = 24000

// Config holds the hook/loop knobs a harness editor may tune.
type Config struct {
	ReasoningEffort   string   `json:"reasoning_effort"`    // executor turns
	PlannerEffort     string   `json:"planner_effort"`      // first turn
	MaxTokens         int      `json:"max_tokens"`          // per turn
	MaxTurns          int      `json:"max_turns"`           // per run
	MaxWallMinutes    int      `json:"max_wall_minutes"`    // per run
	CompactAtTokens   int      `json:"compact_at_tokens"`   // prompt size that triggers compaction
	ToolCapTokens     int      `json:"tool_cap_tokens"`     // inline tool output cap
	MaxFinishAttempts int      `json:"max_finish_attempts"` // failed acceptance checks before giving up
	MaxNudges         int      `json:"max_nudges"`          // consecutive no-tool-call turns tolerated
	SubAgentMaxTurns  int      `json:"subagent_max_turns"`  // per delegate
	SubAgentTools     []string `json:"subagent_tools"`      // tool subset for delegates
	LoopRepeat        int      `json:"loop_repeat"`         // identical calls within 8 that trigger a hint
	LoopNoProgress    int      `json:"loop_no_progress"`    // calls without progress that trigger a hint
	VerifyWrites      bool     `json:"verify_writes"`       // run a syntax check after writes (gofmt -l for .go)
	ErrorHints        []Hint   `json:"error_hints"`         // targeted hints appended after recurring error signatures
	Temperature       *float64 `json:"temperature,omitempty"`
	TopP              *float64 `json:"top_p,omitempty"`
	PresencePenalty   *float64 `json:"presence_penalty,omitempty"`
	CheckOnFinish     bool     `json:"check_on_finish"` // run the task acceptance command on finish
	CompactSummaryMax int      `json:"compact_summary_max_tokens"`
}

// Hint is a Harness² "plugin": when a tool result matches Pattern, Text is
// appended to it.
type Hint struct {
	Tool    string `json:"tool"`    // optional tool name filter
	Pattern string `json:"pattern"` // regexp over the tool output
	Text    string `json:"text"`
	re      *regexp.Regexp
}

func (h *Hint) Match(tool, out string) bool {
	if h.Tool != "" && h.Tool != tool {
		return false
	}
	if h.re == nil {
		h.re = regexp.MustCompile(h.Pattern)
	}
	return h.re.MatchString(out)
}

// DefaultConfig is used for fields missing from config.json.
func DefaultConfig() Config {
	return Config{
		ReasoningEffort:   "medium",
		PlannerEffort:     "medium",
		MaxTokens:         16000,
		MaxTurns:          80,
		MaxWallMinutes:    60,
		CompactAtTokens:   88000,
		ToolCapTokens:     4000,
		MaxFinishAttempts: 4,
		MaxNudges:         3,
		SubAgentMaxTurns:  20,
		SubAgentTools:     []string{"read_file", "grep", "list_dir", "run", "finish"},
		LoopRepeat:        3,
		LoopNoProgress:    14,
		VerifyWrites:      true,
		CheckOnFinish:     true,
		CompactSummaryMax: 3000,
	}
}

type Skill struct {
	Name        string
	Description string
}

type Harness struct {
	Dir       string
	System    string // always-on text
	Hash      string // hash of the whole tree
	Config    Config
	Skills    []Skill
	SubAgents map[string]string // name -> AGENT.md
	Scripts   []string
}

var alwaysOn = []string{"systemprompt.md", "guardrails.md", "LongTermMEMORY.md", "tool_descriptions/tool_guidance.md"}

// Load reads a harness tree.
func Load(dir string) (*Harness, error) {
	h := &Harness{Dir: dir, SubAgents: map[string]string{}, Config: DefaultConfig()}
	var parts []string
	for _, f := range alwaysOn {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			parts = append(parts, s)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		if err := json.Unmarshal(b, &h.Config); err != nil {
			return nil, fmt.Errorf("config.json: %w", err)
		}
	}
	for i := range h.Config.ErrorHints {
		re, err := regexp.Compile(h.Config.ErrorHints[i].Pattern)
		if err != nil {
			return nil, fmt.Errorf("config.json error_hints[%d]: %w", i, err)
		}
		h.Config.ErrorHints[i].re = re
	}
	ents, _ := os.ReadDir(filepath.Join(dir, "skills"))
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "skills", e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		h.Skills = append(h.Skills, Skill{Name: e.Name(), Description: skillDescription(string(b))})
	}
	ents, _ = os.ReadDir(filepath.Join(dir, "sub_agents"))
	for _, e := range ents {
		if b, err := os.ReadFile(filepath.Join(dir, "sub_agents", e.Name(), "AGENT.md")); err == nil {
			h.SubAgents[e.Name()] = string(b)
		}
	}
	ents, _ = os.ReadDir(filepath.Join(dir, "scripts"))
	for _, e := range ents {
		if !e.IsDir() {
			h.Scripts = append(h.Scripts, e.Name())
		}
	}
	if len(h.Skills) > 0 {
		var b strings.Builder
		b.WriteString("# Skills\nSkills are detailed playbooks. Read the SKILL.md with read_file BEFORE starting work that matches its trigger.\n")
		for _, s := range h.Skills {
			fmt.Fprintf(&b, "- %s (.qh/harness/skills/%s/SKILL.md): %s\n", s.Name, s.Name, s.Description)
		}
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	h.System = strings.Join(parts, "\n\n")
	hash, err := treeHash(dir)
	if err != nil {
		return nil, err
	}
	h.Hash = hash
	return h, nil
}

// skillDescription takes the "description:" frontmatter line or the first
// non-heading line of SKILL.md.
func skillDescription(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "description:"))
		}
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "#") && t != "---" && !strings.Contains(t, ":") {
			return t
		}
	}
	return ""
}

func treeHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hs := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(dir, f)
		fmt.Fprintf(hs, "%s\x00%d\x00", rel, len(b))
		hs.Write(b)
	}
	return hex.EncodeToString(hs.Sum(nil))[:12], nil
}

var scriptRef = regexp.MustCompile(`scripts/([A-Za-z0-9_.\-]+)`)

// Validate applies the Harness² tree rules. It returns a list of problems.
func (h *Harness) Validate() []string {
	var probs []string
	if n := len(h.System); n > MaxAlwaysOnChars {
		probs = append(probs, fmt.Sprintf("always-on text is %d chars (cap %d)", n, MaxAlwaysOnChars))
	}
	if strings.Contains(h.System, "{{") || strings.Contains(h.System, "{%") {
		probs = append(probs, "always-on text contains template braces")
	}
	// Every skill must be named verbatim in hand-written always-on text
	// (systemprompt.md or guardrails.md), not only in the generated index.
	var hand strings.Builder
	for _, f := range []string{"systemprompt.md", "guardrails.md"} {
		b, _ := os.ReadFile(filepath.Join(h.Dir, f))
		hand.Write(b)
	}
	for _, s := range h.Skills {
		if !strings.Contains(hand.String(), s.Name) {
			probs = append(probs, fmt.Sprintf("skill %q is not named in systemprompt.md or guardrails.md", s.Name))
		}
		if s.Description == "" {
			probs = append(probs, fmt.Sprintf("skill %q has no description line", s.Name))
		}
	}
	// Every referenced script must exist.
	have := map[string]bool{}
	for _, s := range h.Scripts {
		have[s] = true
	}
	var all strings.Builder
	all.WriteString(h.System)
	_ = filepath.WalkDir(filepath.Join(h.Dir, "skills"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			b, _ := os.ReadFile(p)
			all.Write(b)
		}
		return nil
	})
	for _, m := range scriptRef.FindAllStringSubmatch(all.String(), -1) {
		name := strings.TrimRight(m[1], ".")
		if !have[name] {
			// Skill-local scripts are allowed: skills/<x>/scripts/<name>.
			if !skillLocalScript(h.Dir, name) {
				probs = append(probs, fmt.Sprintf("referenced script %q does not exist", name))
			}
		}
	}
	return probs
}

func skillLocalScript(dir, name string) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(dir, "skills"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == name {
			found = true
		}
		return nil
	})
	return found
}

// Materialize copies skills, scripts and sub-agents into <workdir>/.qh/harness
// so the executor can read and run them with its normal tools.
func (h *Harness) Materialize(workdir string) error {
	dst := filepath.Join(workdir, ".qh", "harness")
	for _, sub := range []string{"skills", "scripts", "sub_agents"} {
		src := filepath.Join(h.Dir, sub)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(h.Dir, p)
			target := filepath.Join(dst, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			info, _ := d.Info()
			mode := os.FileMode(0o644)
			if info != nil && info.Mode()&0o111 != 0 {
				mode = 0o755
			}
			return os.WriteFile(target, b, mode)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
