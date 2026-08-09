# Example project documentation demo

This demo shows how a small project can keep architecture, conventions, and
decisions close to its code. The example program checks the minimum shape of a
claim and source pair before editorial review.

The repository uses English Wikipedia policies as public reference material.
It is an educational demo, not a Wikipedia tool or a substitute for Wikipedia
policy.

## Try the project

Install Open Knowledge if `okn version` does not work:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Then, run the code and knowledge checks:

```sh
npm test
okn validate Wiki
okn search Wiki "what context affects citation validation?"
okn view Wiki
```

The `Wiki/` folder is an Open Knowledge Format 0.2 bundle. Its
`.openknowledge.toml` file also permits a static demo export:

```sh
okn export html --out site Wiki
```

## Connect after publication

After this folder becomes the `openknowledge-sh/example-project-docs`
repository, connect it as a read-only knowledge base:

```sh
okn connect https://github.com/openknowledge-sh/example-project-docs.git \
  --git-subdir Wiki \
  --as example-project-docs
okn view example-project-docs
```

## Repository contents

```text
.
├── AGENTS.md
├── src/citation-validator.mjs
├── test/citation-validator.test.mjs
└── Wiki/
    ├── architecture/editorial-workflow.md
    ├── conventions/citations-and-sources.md
    ├── decisions/0001-demo-scope.md
    └── sources/index.md
```

## License and attribution

The demo code and original documentation use Apache-2.0, matching the Open
Knowledge repository. Linked Wikipedia policy pages remain under their own
terms.
