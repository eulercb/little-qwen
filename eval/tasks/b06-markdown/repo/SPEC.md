# Markdown subset → HTML

`ToHTML(src)` converts the input to HTML. Input lines are split on `\n` (a trailing `\r` is removed from each line).
Tabs are not special. Output: each block element is followed by `\n`; nothing else is added between blocks.

## Blocks
Blocks are recognised line by line, in this priority order:

1. **Blank line** (empty or only spaces): ends the current paragraph / list / blockquote.
2. **Fenced code block**: a line starting with three backticks, optionally followed by an info word
   (e.g. ```` ```go ````). It ends at the next line that is exactly three backticks (after trimming trailing spaces)
   or at end of input. Content lines are copied verbatim (HTML-escaped, no inline processing), each followed by `\n`.
   Output: `<pre><code>CONTENT</code></pre>` or, with an info word, `<pre><code class="language-go">CONTENT</code></pre>`.
3. **Heading**: 1–6 `#` followed by a space, then the text. Trailing spaces and a trailing run of `#` (preceded by a space)
   are removed. Output `<hN>INLINE</hN>`. `#foo` (no space) is not a heading. `#` alone or `# ` gives `<h1></h1>`.
4. **Horizontal rule**: a line consisting only of 3 or more `-`, or 3 or more `*`, or 3 or more `_` (no spaces). Output `<hr />`.
5. **Blockquote**: lines starting with `>` (optionally followed by one space, which is removed). Consecutive `>` lines
   form one blockquote whose content (the lines with the marker removed) is itself rendered with these block rules
   (recursively). Output `<blockquote>\nINNER</blockquote>` where INNER is the rendered content (which ends in `\n`).
6. **List item**: `- `, `* ` or `+ ` (unordered) or `N. ` with N one or more digits (ordered), at indentation 0.
   Consecutive items of the same kind (unordered/ordered) form one list; a different kind starts a new list.
   A line indented by 2 or more spaces right after an item (or after another such line) is a continuation:
   if it (with the indentation removed — remove exactly as many leading spaces as it has, up to 4) starts a list item,
   it belongs to a nested list inside the current item; otherwise the line, with all its leading spaces removed, is joined to the item text with `\n`.
   Nested lists follow the same rules relative to their own indentation (only one nesting level needs to be supported).
   Output: `<ul>\n<li>INLINE</li>\n...</ul>` / `<ol>\n...</ol>`; an ordered list whose first number is not 1 gets
   `<ol start="N">`. An item with a nested list renders as `<li>INLINE\n<ul>\n...</ul>\n</li>`.
   List items are always "tight": no `<p>` inside `<li>`.
7. **Paragraph**: any other line. Consecutive paragraph lines are joined with `\n` (leading spaces of each line removed)
   and rendered as `<p>INLINE</p>`. A heading, fence, rule, blockquote or list item line ends the paragraph
   (so a `- ` line directly after paragraph text starts a list).

## Inline (applied to heading, paragraph and list-item text)
Processed left to right:

- **Backslash escape**: `\` followed by one of ``\`*_{}[]()#+-.!<>`` outputs that character literally (escaped for HTML).
  A `\` at the very end of a line (not the last line of the block) is a **hard break**: output `<br />` followed by the newline.
- **Hard break**: a line ending in two or more spaces (not the last line of the block) → the spaces are removed and
  `<br />` is inserted before the newline.
- **Code span**: text between a run of N backticks and the next run of exactly N backticks. The content is HTML-escaped,
  not processed further, and if it both starts and ends with a space (and is not only spaces), one space is removed
  from each end. Output `<code>…</code>`. A backtick run with no matching closer is literal text.
- **Autolink**: `<` + `http://` or `https://` URL without spaces or `<>` + `>` → `<a href="URL">URL</a>`.
- **Link**: `[text](url)` or `[text](url "title")`; `text` is processed as inline (no nested links); `url` has no spaces.
  Output `<a href="url">text</a>` / `<a href="url" title="title">text</a>`. Anything that doesn't complete this
  pattern is literal text.
- **Strong**: `**text**` or `__text__` → `<strong>…</strong>`. **Emphasis**: `*text*` or `_text_` → `<em>…</em>`.
  An opening delimiter must be followed by a non-space character and a closing delimiter must be preceded by a
  non-space character. `_` delimiters do not open or close inside a word (`snake_case_name` stays literal);
  `*` may (`a*b*c` → `a<em>b</em>c`). Strong and emphasis nest (`**a *b* c**`). Unmatched delimiters are literal.
- **Trailing spaces** at the end of every line of a block are removed (after hard-break detection).
- **HTML escaping** of all literal text: `&` → `&amp;`, `<` → `&lt;`, `>` → `&gt;`, `"` → `&quot;`. Attribute values
  (href, title) are escaped the same way.

## Examples
Input:
```
# Title #

Some *emphasis* and **strong** text
with `code` and a [link](http://x.io "T").

- one
- two
  - nested
3. three
```
Output:
```
<h1>Title</h1>
<p>Some <em>emphasis</em> and <strong>strong</strong> text
with <code>code</code> and a <a href="http://x.io" title="T">link</a>.</p>
<ul>
<li>one</li>
<li>two
<ul>
<li>nested</li>
</ul>
</li>
</ul>
<ol start="3">
<li>three</li>
</ol>
```
