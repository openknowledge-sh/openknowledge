# Example research notes demo

This demo asks how a citation-review workflow should preserve evidence. It
keeps the research question, raw source notes, synthesis, limits, and open
questions in a small Open Knowledge bundle.

The research summarizes public English Wikipedia policy and help pages. It is
an educational example, not a Wikipedia policy document or an empirical user
study.

## Try the knowledge base

Install Open Knowledge if `okn version` does not work:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Then, verify and explore the research:

```sh
okn validate Wiki
okn search Wiki "what evidence supports the citation workflow?"
okn view Wiki
```

Create a static copy of the knowledge base:

```sh
okn export html --out site Wiki
```

## Connect after publication

After this folder becomes the `openknowledge-sh/example-research-notes`
repository, connect it as a read-only knowledge base:

```sh
okn connect https://github.com/openknowledge-sh/example-research-notes.git \
  --git-subdir Wiki \
  --as example-research-notes
okn view example-research-notes
```

## Research structure

```text
Wiki/
├── research/
│   ├── question.md
│   └── citation-workflow.md
└── sources/
    ├── verifiability-policy.md
    ├── reliable-sources-guideline.md
    └── citing-sources-guideline.md
```

Each source note links to the original page. The synthesis uses structured
OKF source metadata and Markdown footnotes to connect findings to evidence.

## License and attribution

The original demo documentation uses Apache-2.0, matching the Open Knowledge
repository. Linked Wikipedia pages remain under their own terms.
