---
type: Feature Documentation
title: Citation review
description: Current behavior of the demo citation-review function.
tags: ["feature", "citations", "current-behavior"]
---

# Citation review

`reviewCitation` checks whether a claim and source pair is ready for editorial
review. The function does not decide whether the source supports the claim.

## Input

Pass an object with `claim` and `sourceUrl` string fields.

## Output

The function returns one stable reason code:

| Code | Meaning |
| --- | --- |
| `claim-required` | The claim is empty. |
| `source-url-invalid` | The source is not a URL. |
| `source-https-required` | The source URL does not use HTTPS. |
| `ready-for-editor-review` | The structural checks pass. |

A successful result includes the trimmed claim and normalized source URL.

## Limits

The function does not assess reliability, direct support, neutrality, due
weight, or consensus. A person must make those decisions.

## Source anchors

- `src/citation-review.mjs`
- `test/citation-review.test.mjs`
- [Product changelog](../changelog/editor-helper.md)
