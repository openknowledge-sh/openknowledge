---
type: Example
title: Syntax Highlighting Examples
description: Fenced code blocks that test local viewer syntax colors in supported languages.
tags: [openknowledge, viewer, syntax, examples]
timestamp: 2026-06-20T00:00:00Z
---

# Syntax Highlighting Examples

Open this page with `okn view Wiki`. Compare fenced code blocks in
the languages that the built-in highlighter supports.

## Shell

```shell
# Build and validate the wiki
set -euo pipefail
GOCACHE=/private/tmp/openknowledge-go-build-cache pnpm test:cli
target="Wiki"

okn validate "$target"
okn export html --out ./packages/web/dist/wiki ${target}
```

## Go

```go
package main

import "fmt"

type Note struct {
	Path  string
	Score int
}

func main() {
	note := Note{Path: "examples/syntax-highlighting.md", Score: 42}
	fmt.Printf("open %s with score %d\n", note.Path, note.Score)
}
```

## TypeScript

```typescript
type Result = {
	path: string;
	score: number;
};

export async function search(query: string): Promise<Result[]> {
	const response = await fetch("/api/search?q=" + encodeURIComponent(query));
	const payload = await response.json();
	return payload.results ?? [];
}
```

## Python

```python
# Keep the example intentionally small.
from dataclasses import dataclass

@dataclass
class Bundle:
    path: str
    valid: bool = True

def describe(bundle: Bundle) -> str:
    return f"{bundle.path}: {bundle.valid}"
```

## JSON

```json
{
  "title": "Syntax Highlighting Examples",
  "published": true,
  "weights": [1, 3, 5],
  "theme": null
}
```

## YAML

```yaml
# Open Knowledge theme settings
html:
  theme:
    name: landing
    stylesheet: assets/openknowledge-site.css
  source:
    github_base: "https://github.com/openknowledge-sh/openknowledge/blob/main"
```

## CSS

```css
/* A tiny accent override */
:root {
  --ok-color-accent: #8db5dc;
  --ok-note-panel-default-width: min(calc(65ch + 68px), calc(100vw - 44px));
}
```

## SQL

```sql
-- Find recently updated examples
select path, title, updated_at
from wiki_pages
where path like 'examples/%'
order by updated_at desc
limit 10;
```
