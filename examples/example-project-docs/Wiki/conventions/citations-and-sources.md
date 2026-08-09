---
type: Convention
title: Citation and source conventions
description: Project rules for claim and source pairs in the demo.
tags: ["conventions", "citations", "sources"]
sources:
  - id: wikipedia-verifiability
    resource: https://en.wikipedia.org/wiki/Wikipedia:Verifiability
    title: Wikipedia Verifiability policy
  - id: wikipedia-reliable-sources
    resource: https://en.wikipedia.org/wiki/Wikipedia:Reliable_sources
    title: Wikipedia Reliable sources guideline
  - id: wikipedia-citing-sources
    resource: https://en.wikipedia.org/wiki/Wikipedia:Citing_sources
    title: Wikipedia Citing sources guideline
---

# Citation and source conventions

Use these project rules when you change the citation validator or its tests.
They summarize the demo contract. They do not replace Wikipedia policy.

## Code rules

- Keep structural validation deterministic.
- Return a stable reason code for each rejected value.
- Do not make source reliability decisions in code.
- Add a test for each new reason code.

## Review rules

- Keep the claim and its source together.
- Check that the cited source directly supports the claim.
- Treat source reliability as dependent on the source and claim context.
- Record uncertainty for a reviewer instead of inventing support.

Wikipedia states that challenged or likely challenged claims need inline
citations to reliable sources that directly support them.[^wikipedia-verifiability]
Citation formatting alone does not establish source reliability.[^wikipedia-citing-sources]

[^wikipedia-verifiability]: [Wikipedia Verifiability policy](https://en.wikipedia.org/wiki/Wikipedia:Verifiability)
[^wikipedia-citing-sources]: [Wikipedia Citing sources guideline](https://en.wikipedia.org/wiki/Wikipedia:Citing_sources)

## Related pages

- [Editorial workflow](../architecture/editorial-workflow.md)
- [Demo scope decision](../decisions/0001-demo-scope.md)
