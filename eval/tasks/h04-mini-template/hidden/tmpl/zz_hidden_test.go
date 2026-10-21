package tmpl

import (
	"strings"
	"testing"
)

func TestHiddenRender(t *testing.T) {
	data := map[string]any{
		"name": "  Ada  ", "n": 42, "f": 1.5, "ok": true, "none": nil, "html": `<a href="x">'&'</a>`,
		"user": map[string]any{"name": "bob", "addr": map[string]any{"city": "Lisbon"}},
		"long": "héllo wörld",
	}
	cases := map[string]string{
		"Hi {{name}}!":                              "Hi   Ada  !",
		"{{ name | trim | upper }}":                 "ADA",
		"{{n}}/{{f}}/{{ok}}":                        "42/1.5/true",
		"[{{none}}][{{missing}}]":                   "[][]",
		`{{ missing | default:"anon" }}`:            "anon",
		`{{ name | default:"x" | trim }}`:           "Ada",
		"{{user.name}} from {{user.addr.city}}":     "bob from Lisbon",
		`{{ long | truncate:"5" }}`:                 "héllo…",
		`{{ long | truncate:"50" }}`:                "héllo wörld",
		`{{ long | truncate:"0" }}`:                 "…",
		"{{ html | html }}":                         "&lt;a href=&#34;x&#34;&gt;&#39;&amp;&#39;&lt;/a&gt;",
		"{{{{not a tag}}":                           "{{not a tag}}",
		`{{ missing | default:"say \"hi\" \\ |" }}`: `say "hi" \ |`,
		"plain text":                                "plain text",
		`{{name|trim|lower}}`:                       "ada",
	}
	for src, want := range cases {
		got, err := Render(src, data)
		if err != nil || got != want {
			t.Errorf("Render(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

func TestHiddenErrors(t *testing.T) {
	data := map[string]any{"s": "x", "user": map[string]any{"name": "bob"}}
	cases := map[string]string{
		"{{ s | shout }}":          "shout",
		"{{ s | default }}":        "",
		`{{ s | upper:"x" }}`:      "",
		`{{ s | truncate:"abc" }}`: "",
		`{{ s | truncate:"-1" }}`:  "",
		"oops {{ s ":               "",
		"{{ }}":                    "",
		"{{ 9lives }}":             "",
		"{{ user.name.first }}":    "user.name.first",
	}
	for src, mention := range cases {
		out, err := Render(src, data)
		if err == nil || out != "" {
			t.Errorf("Render(%q) = %q, %v; want error", src, out, err)
			continue
		}
		if mention != "" && !strings.Contains(err.Error(), mention) {
			t.Errorf("Render(%q) error %q should mention %q", src, err, mention)
		}
	}
}
