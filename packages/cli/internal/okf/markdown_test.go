package okf

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSupportedSyntax(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		required  []string
		forbidden []string
	}{
		{
			name: "headings paragraphs and escaping",
			input: strings.Join([]string{
				"# Title",
				"## Section",
				"Plain text with <tag> & value.",
				"continued on next line.",
			}, "\n"),
			required: []string{
				"<h1>Title</h1>",
				"<h2>Section</h2>",
				"<p>Plain text with &lt;tag&gt; &amp; value. continued on next line.</p>",
			},
			forbidden: []string{"<tag>"},
		},
		{
			name:  "inline emphasis and code",
			input: "Use **bold**, *italic*, and `**literal** <tag>`.",
			required: []string{
				"<strong>bold</strong>",
				"<em>italic</em>",
				"<code>**literal** &lt;tag&gt;</code>",
			},
			forbidden: []string{"**bold**", "*italic*"},
		},
		{
			name:  "links with custom resolver",
			input: "Read [Setup](guides/setup.md), [Anchor](#top), and [External](https://openknowledge.sh).",
			required: []string{
				`<a href="/resolved/guides/setup.md">Setup</a>`,
				`<a href="/resolved/#top">Anchor</a>`,
				`<a href="/resolved/https://openknowledge.sh">External</a>`,
			},
			forbidden: []string{"[Setup]", "(guides/setup.md)"},
		},
		{
			name:  "links with inline code labels",
			input: "See [React `useEffect`](https://react.dev/reference/react/useEffect) and `[literal](skip.md)`.",
			required: []string{
				`<a href="/resolved/https://react.dev/reference/react/useEffect">React <code>useEffect</code></a>`,
				`<code>[literal](skip.md)</code>`,
			},
			forbidden: []string{
				"[React",
				`](https://react.dev/reference/react/useEffect)`,
				`href="/resolved/skip.md"`,
			},
		},
		{
			name: "unordered and ordered lists",
			input: strings.Join([]string{
				"- **Readable** by humans.",
				"* Portable across tools.",
				"",
				"1. First item.",
				"2) Second item.",
			}, "\n"),
			required: []string{
				"<ul>",
				"<li><strong>Readable</strong> by humans.</li>",
				"<li>Portable across tools.</li>",
				"</ul>",
				"<ol>",
				"<li>First item.</li>",
				"<li>Second item.</li>",
				"</ol>",
			},
			forbidden: []string{"- **Readable**", "2) Second"},
		},
		{
			name: "wrapped list item continuations",
			input: strings.Join([]string{
				"* Browse local Markdown links as adjacent panels without leaving the current",
				"  context.",
				"* Move across multi-panel stacks with the bottom rail, rail keyboard controls,",
				"  by dragging the workspace gaps, or by holding `Space` and dragging sideways.",
			}, "\n"),
			required: []string{
				"<ul>",
				"<li>Browse local Markdown links as adjacent panels without leaving the current context.</li>",
				"<li>Move across multi-panel stacks with the bottom rail, rail keyboard controls, by dragging the workspace gaps, or by holding <code>Space</code> and dragging sideways.</li>",
				"</ul>",
			},
			forbidden: []string{
				"<p>context.</p>",
				"<p>by dragging the workspace gaps",
			},
		},
		{
			name: "blockquotes and horizontal rules",
			input: strings.Join([]string{
				"> This is a **pinned** copy.",
				"> It is *unofficial* tooling.",
				"",
				"---",
			}, "\n"),
			required: []string{
				"<blockquote>",
				"<p>This is a <strong>pinned</strong> copy. It is <em>unofficial</em> tooling.</p>",
				"</blockquote>",
				"<hr>",
			},
			forbidden: []string{"> This", "**pinned**", "*unofficial*", "<p>---</p>"},
		},
		{
			name: "agent maintenance footer marker",
			input: strings.Join([]string{
				"# Page",
				"",
				"---",
				"",
				"<!-- okf-footer: agent-maintenance -->",
				"",
				"> **Source anchors**",
				">",
				"> - `packages/cli/main.go`",
			}, "\n"),
			required: []string{
				`<div class="ok-agent-footer">`,
				"<blockquote>",
				"<strong>Source anchors</strong>",
				"<li><code>packages/cli/main.go</code></li>",
				"</div>",
			},
			forbidden: []string{
				"okf-footer: agent-maintenance",
				"&lt;!--",
				"<hr>",
			},
		},
		{
			name: "fenced code",
			input: strings.Join([]string{
				"```go",
				"package main",
				"func main() {",
				"  println(\"<tag>\")",
				"}",
				"<script>",
				"```",
			}, "\n"),
			required: []string{
				`<pre class="code-block language-go" data-language="go"><code>`,
				`<span class="tok-keyword">package</span> main`,
				`<span class="tok-keyword">func</span> main()`,
				`<span class="tok-string">&#34;&lt;tag&gt;&#34;</span>`,
				"&lt;script&gt;",
			},
			forbidden: []string{"<h1>Not a heading</h1>", "<script>"},
		},
		{
			name: "shell highlighting",
			input: strings.Join([]string{
				"```sh",
				"# Build docs",
				"GOCACHE=/tmp/openknowledge pnpm test:cli",
				"target=\"Wiki\"",
				"openknowledge validate $target --strict",
				"openknowledge to html --out ./dist ${target}",
				"```",
			}, "\n"),
			required: []string{
				`<pre class="code-block language-shell" data-language="shell"><code>`,
				`<span class="tok-comment"># Build docs</span>`,
				`<span class="tok-variable">GOCACHE</span>=/tmp/openknowledge <span class="tok-command">pnpm</span> test:cli`,
				`<span class="tok-variable">target</span>=<span class="tok-string">&#34;Wiki&#34;</span>`,
				`<span class="tok-command">openknowledge</span> validate <span class="tok-variable">$target</span> <span class="tok-flag">--strict</span>`,
				`<span class="tok-command">openknowledge</span> to html <span class="tok-flag">--out</span> ./dist <span class="tok-variable">${target}</span>`,
			},
		},
		{
			name: "tables",
			input: strings.Join([]string{
				"| Field | Purpose | Score |",
				"| :--- | --- | ---: |",
				"| `type` | **Required** concept kind. | 10 |",
				"| `title` | Optional display name. | 5 |",
			}, "\n"),
			required: []string{
				`<div class="ok-table-wrap" data-ok-table-wrap>`,
				`<div class="ok-table-scroller">`,
				`<table class="ok-table" data-ok-table>`,
				"<thead>",
				`<th scope="col" data-align="left">Field</th>`,
				`<th scope="col">Purpose</th>`,
				`<th scope="col" data-align="right">Score</th>`,
				"<tbody>",
				`<td data-align="left"><code>type</code></td>`,
				"<td><strong>Required</strong> concept kind.</td>",
				`<td data-align="right">10</td>`,
				`<td data-align="left"><code>title</code></td>`,
				"</table>",
				"</div>",
			},
			forbidden: []string{"| :--- |", "**Required**"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			html := RenderMarkdown(test.input, "index.md", func(_ string, href string) string {
				return "/resolved/" + href
			})
			assertContainsAll(t, html, test.required)
			assertContainsNone(t, html, test.forbidden)
		})
	}
}

func TestRenderMarkdownHandlesCommonSpecSyntax(t *testing.T) {
	input := strings.Join([]string{
		"> This is a **pinned** upstream copy",
		"> with *portable* Markdown.",
		"",
		"---",
		"",
		"1. Define a **universal** format.",
		"2. Keep `**literal**` code intact.",
		"",
		"- **Readable** by humans.",
		"",
		"| Field | Purpose |",
		"| --- | --- |",
		"| `type` | **Required** concept kind. |",
	}, "\n")

	html := RenderMarkdown(input, "SPEC.md", nil)

	required := []string{
		"<blockquote>",
		"<strong>pinned</strong>",
		"<em>portable</em>",
		"<hr>",
		"<ol>",
		"<li>Define a <strong>universal</strong> format.</li>",
		"<code>**literal**</code>",
		"<ul>",
		"<li><strong>Readable</strong> by humans.</li>",
		`<table class="ok-table" data-ok-table>`,
		`<th scope="col">Field</th>`,
		"<td><code>type</code></td>",
		"<td><strong>Required</strong> concept kind.</td>",
	}
	for _, expected := range required {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected rendered markdown to contain %q:\n%s", expected, html)
		}
	}

	for _, raw := range []string{"> This is", "**pinned**", "*portable*", "\n---\n"} {
		if strings.Contains(html, raw) {
			t.Fatalf("expected raw markdown %q to be rendered away:\n%s", raw, html)
		}
	}
}

func TestRenderMarkdownEmbeddedSpecDoesNotLeakCommonMarkdownSyntax(t *testing.T) {
	_, body, err := splitFrontmatter(specDocument())
	if err != nil {
		t.Fatal(err)
	}

	html := RenderMarkdown(body, "SPEC.md", StaticHTMLLink)

	assertContainsAll(t, html, []string{
		"<blockquote>",
		"<h1>Open Knowledge Format (OKF)</h1>",
		"<p><strong>Version 0.1 — Draft</strong></p>",
		"representing <em>knowledge</em>",
		"<code>cat</code>",
		"<code>git clone</code>",
		"<hr>",
		"<li><strong>Readable</strong> by humans without tooling.</li>",
		"<ol>",
		"<li>Define a universal format that <strong>enrichment agents</strong> can write into.</li>",
		`<table class="ok-table" data-ok-table>`,
		`<th scope="col">Filename</th>`,
		"<td><code>index.md</code></td>",
		`<pre class="code-block`,
		`<span class="tok-comment"># REQUIRED</span>`,
	})
	assertContainsNone(t, html, []string{
		"&gt; This is a pinned upstream copy",
		"**Version 0.1",
		"*knowledge*",
		"<p>---</p>",
		"| Filename",
		"|--------------|",
	})
}

func assertContainsAll(t *testing.T, html string, expected []string) {
	t.Helper()
	for _, value := range expected {
		if !strings.Contains(html, value) {
			t.Fatalf("expected rendered markdown to contain %q:\n%s", value, html)
		}
	}
}

func assertContainsNone(t *testing.T, html string, forbidden []string) {
	t.Helper()
	for _, value := range forbidden {
		if strings.Contains(html, value) {
			t.Fatalf("expected rendered markdown not to contain %q:\n%s", value, html)
		}
	}
}
