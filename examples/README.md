# Demo knowledge bases

These projects implement the three use cases from
[openknowledge.sh/use-cases](https://openknowledge.sh/use-cases/).


- [`example-project-docs`](example-project-docs/README.md) keeps architecture,
  conventions, and decisions beside a small citation checker.
- [`example-changelog`](example-changelog/README.md) keeps current product
  documentation separate from release history.
- [`example-research-notes`](example-research-notes/README.md) keeps source
  notes separate from a research summary.

The demos use English Wikipedia policy pages as public source material.

Run all demo checks from the Open Knowledge repository root:

```sh
pnpm test:demos
```
