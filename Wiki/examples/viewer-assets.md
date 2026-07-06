---
type: Example
title: Viewer Asset Examples
description: Small bundled files for manually testing code highlighting and browser PDF viewing.
tags: [openknowledge, viewer, assets, examples]
timestamp: 2026-06-19T00:00:00Z
---

# Viewer Asset Examples

Use this page with `openknowledge view Wiki` to test local asset handling in
the viewer.

## Files

* [Go code example](hello-viewer.go) - should open as a syntax-highlighted code preview.
* [PDF asset example](browser-preview.pdf) - should open through the browser PDF viewer.

## Inline Code Fence

```go
package main

import "fmt"

func main() {
	fmt.Println("Open Knowledge viewer highlighting works")
}
```
