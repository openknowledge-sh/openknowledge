---
type: Changelog
title: Citation Review Demo Changelog
description: User-visible changes to the citation-review example.
tags: ["changelog", "release-notes", "evidence"]
---

# Citation Review Demo Changelog

## Unreleased

No user-visible changes are pending.

## 0.2.0

### Review results

- Citation review now returns stable reason codes for each result.
- Users can distinguish an invalid URL from a non-HTTPS source.
- Successful results include a normalized source URL.
- Source: `src/citation-review.mjs`, `test/citation-review.test.mjs`.
- Docs: `Wiki/features/citation-review.md`.

## 0.1.0

### Citation checks

- Added the first claim and source structural check.
- The check requires claim text and a valid source URL.
- Source: `src/citation-review.mjs`, `test/citation-review.test.mjs`.
- Docs: `Wiki/features/citation-review.md`.
