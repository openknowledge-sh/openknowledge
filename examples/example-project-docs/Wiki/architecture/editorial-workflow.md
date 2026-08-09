---
type: Architecture
title: Editorial workflow
description: How a claim and source move through the demo citation check and human review.
tags: ["architecture", "citations", "workflow"]
sources:
  - id: wikipedia-verifiability
    resource: https://en.wikipedia.org/wiki/Wikipedia:Verifiability
    title: Wikipedia Verifiability policy
  - id: wikipedia-consensus
    resource: https://en.wikipedia.org/wiki/Wikipedia:Consensus
    title: Wikipedia Consensus policy
---

# Editorial workflow

The demo performs a small structural check before a person reviews a proposed
Wikipedia edit. It does not decide whether a source is reliable.

```mermaid
flowchart LR
    A["Draft claim"] --> B["Add source URL"]
    B --> C["Run structural check"]
    C --> D["Review source support"]
    D --> E["Discuss and revise"]
```

## Component boundary

`src/citation-validator.mjs` checks three facts:

- The claim contains text.
- The source value is a URL.
- The source URL uses HTTPS.

The reviewer checks whether the source directly supports the claim. The
reviewer also applies current Wikipedia policies and community consensus.

## Change rule

Update this page when a code change moves responsibility between the
validator, reviewer, or community process.

## Related pages

- [Citation and source conventions](../conventions/citations-and-sources.md)
- [Demo scope decision](../decisions/0001-demo-scope.md)
