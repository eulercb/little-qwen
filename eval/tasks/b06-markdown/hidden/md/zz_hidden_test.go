package md

import "testing"

func zzhCheck(t *testing.T, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		if got := ToHTML(c[0]); got != c[1] {
			t.Errorf("ToHTML(%q)\n got %q\nwant %q", c[0], got, c[1])
		}
	}
}

func TestHiddenCode(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"```\na *b*\n<x>\n```", "<pre><code>a *b*\n&lt;x&gt;\n</code></pre>\n"},
		{"```go\nfunc(){}\n```\nafter", "<pre><code class=\"language-go\">func(){}\n</code></pre>\n<p>after</p>\n"},
		{"```\nunterminated", "<pre><code>unterminated\n</code></pre>\n"},
	})
}

func TestHiddenCodeSpans(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"`a*b*`", "<p><code>a*b*</code></p>\n"},
		{"`` a`b ``", "<p><code>a`b</code></p>\n"},
		{"` x `", "<p><code>x</code></p>\n"},
		{"`  `", "<p><code>  </code></p>\n"},
		{"``no close", "<p>``no close</p>\n"},
		{"`<&>`", "<p><code>&lt;&amp;&gt;</code></p>\n"},
		{"\\`not code`", "<p>`not code`</p>\n"},
	})
}

func TestHiddenEmphasis(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"*a* _b_ **c** __d__", "<p><em>a</em> <em>b</em> <strong>c</strong> <strong>d</strong></p>\n"},
		{"**a *b* c**", "<p><strong>a <em>b</em> c</strong></p>\n"},
		{"*a **b** c*", "<p><em>a <strong>b</strong> c</em></p>\n"},
		{"snake_case_name", "<p>snake_case_name</p>\n"},
		{"a*b*c", "<p>a<em>b</em>c</p>\n"},
		{"* not em*", "<ul>\n<li>not em*</li>\n</ul>\n"},
		{"*not em *", "<p>*not em *</p>\n"},
		{"**unclosed", "<p>**unclosed</p>\n"},
		{"_a_b", "<p>_a_b</p>\n"},
		{"x _y_ z", "<p>x <em>y</em> z</p>\n"},
		{"*a\nb*", "<p><em>a\nb</em></p>\n"},
	})
}

func TestHiddenEscapes(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"\\*a\\*", "<p>*a*</p>\n"},
		{"\\\\", "<p>\\</p>\n"},
		{"\\q", "<p>\\q</p>\n"},
		{"a & b < c > \"d\"", "<p>a &amp; b &lt; c &gt; &quot;d&quot;</p>\n"},
		{"\\# not heading", "<p># not heading</p>\n"},
		{"\\_x\\_", "<p>_x_</p>\n"},
	})
}

func TestHiddenHeadings(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"# A", "<h1>A</h1>\n"},
		{"###### Six", "<h6>Six</h6>\n"},
		{"####### seven", "<p>####### seven</p>\n"},
		{"#nospace", "<p>#nospace</p>\n"},
		{"## Trail ##", "<h2>Trail</h2>\n"},
		{"# a # b", "<h1>a # b</h1>\n"},
		{"#", "<h1></h1>\n"},
		{"### x ###   ", "<h3>x</h3>\n"},
	})
}

func TestHiddenLinks(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"[a](http://x)", "<p><a href=\"http://x\">a</a></p>\n"},
		{"[*a*](u \"t\")", "<p><a href=\"u\" title=\"t\"><em>a</em></a></p>\n"},
		{"[a] (u)", "<p>[a] (u)</p>\n"},
		{"[a](u", "<p>[a](u</p>\n"},
		{"<https://e.com/x?y=1&z=2>", "<p><a href=\"https://e.com/x?y=1&amp;z=2\">https://e.com/x?y=1&amp;z=2</a></p>\n"},
		{"<ftp://no>", "<p>&lt;ftp://no&gt;</p>\n"},
		{"[a](u \"t)", "<p>[a](u &quot;t)</p>\n"},
	})
}

func TestHiddenLists(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"- a\n- b", "<ul>\n<li>a</li>\n<li>b</li>\n</ul>\n"},
		{"1. a\n2. b", "<ol>\n<li>a</li>\n<li>b</li>\n</ol>\n"},
		{"7. x\n8. y", "<ol start=\"7\">\n<li>x</li>\n<li>y</li>\n</ol>\n"},
		{"- a\n1. b", "<ul>\n<li>a</li>\n</ul>\n<ol>\n<li>b</li>\n</ol>\n"},
		{"* a\n+ b", "<ul>\n<li>a</li>\n<li>b</li>\n</ul>\n"},
		{"- a\n  more\n- b", "<ul>\n<li>a\nmore</li>\n<li>b</li>\n</ul>\n"},
		{"- a\n  - b\n  - c\n- d", "<ul>\n<li>a\n<ul>\n<li>b</li>\n<li>c</li>\n</ul>\n</li>\n<li>d</li>\n</ul>\n"},
		{"- a\n\n- b", "<ul>\n<li>a</li>\n</ul>\n<ul>\n<li>b</li>\n</ul>\n"},
		{"para\n- item", "<p>para</p>\n<ul>\n<li>item</li>\n</ul>\n"},
		{"- a\n  1. x\n  2. y", "<ul>\n<li>a\n<ol>\n<li>x</li>\n<li>y</li>\n</ol>\n</li>\n</ul>\n"},
		{"-a", "<p>-a</p>\n"},
		{"1.a", "<p>1.a</p>\n"},
		{"- *em* `c`", "<ul>\n<li><em>em</em> <code>c</code></li>\n</ul>\n"},
	})
}

func TestHiddenParagraphs(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"a\nb\n\nc", "<p>a\nb</p>\n<p>c</p>\n"},
		{"  lead\n   spaces", "<p>lead\nspaces</p>\n"},
		{"para\n# head\nmore", "<p>para</p>\n<h1>head</h1>\n<p>more</p>\n"},
		{"x  \ny", "<p>x<br />\ny</p>\n"},
		{"x\\\ny", "<p>x<br />\ny</p>\n"},
		{"x \ny", "<p>x\ny</p>\n"},
		{"last  ", "<p>last</p>\n"},
	})
}

func TestHiddenQuotes(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"> a\n> b", "<blockquote>\n<p>a\nb</p>\n</blockquote>\n"},
		{"> # h\n>\n> p", "<blockquote>\n<h1>h</h1>\n<p>p</p>\n</blockquote>\n"},
		{">no space", "<blockquote>\n<p>no space</p>\n</blockquote>\n"},
		{"> - x\n> - y", "<blockquote>\n<ul>\n<li>x</li>\n<li>y</li>\n</ul>\n</blockquote>\n"},
		{"> a\nb", "<blockquote>\n<p>a</p>\n</blockquote>\n<p>b</p>\n"},
	})
}

func TestHiddenRules(t *testing.T) {
	zzhCheck(t, [][2]string{
		{"---", "<hr />\n"},
		{"***", "<hr />\n"},
		{"___", "<hr />\n"},
		{"--", "<p>--</p>\n"},
		{"- - -", "<ul>\n<li>- -</li>\n</ul>\n"},
		{"a\n---\nb", "<p>a</p>\n<hr />\n<p>b</p>\n"},
	})
}

func TestHiddenSpecExample(t *testing.T) {
	zzhCheck(t, [][2]string{{"# Title #\n\nSome *emphasis* and **strong** text\nwith `code` and a [link](http://x.io \"T\").\n\n- one\n- two\n  - nested\n3. three\n", "<h1>Title</h1>\n<p>Some <em>emphasis</em> and <strong>strong</strong> text\nwith <code>code</code> and a <a href=\"http://x.io\" title=\"T\">link</a>.</p>\n<ul>\n<li>one</li>\n<li>two\n<ul>\n<li>nested</li>\n</ul>\n</li>\n</ul>\n<ol start=\"3\">\n<li>three</li>\n</ol>\n"}})
}
