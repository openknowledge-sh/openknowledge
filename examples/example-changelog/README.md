# Example changelog demo

This demo shows how a project can keep current product documentation separate
from release history. A small citation-review program gives every changelog
entry real code, test, and documentation anchors.

The repository uses English Wikipedia editing as an example context. It is not
affiliated with or approved by the Wikimedia Foundation or the Wikipedia
community.

## Try the project

Install Open Knowledge if `okn version` does not work:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Then, run the code and knowledge checks:

```sh
npm test
okn validate Wiki
okn search Wiki "what changed for invalid source URLs?"
okn view Wiki
```

Create a static copy of the knowledge base:

```sh
okn export html --out site Wiki
```

## Connect after publication

After this folder becomes the `openknowledge-sh/example-changelog`
repository, connect it as a read-only knowledge base:

```sh
okn connect https://github.com/openknowledge-sh/example-changelog.git \
  --git-subdir Wiki \
  --as example-changelog
okn view example-changelog
```

## What the demo proves

- `Wiki/features/citation-review.md` describes current behavior.
- `Wiki/changelog/editor-helper.md` records user-visible changes.
- `Wiki/log.md` records changes to the knowledge base itself.
- Each product entry links to the code, tests, and current guide.

## License and attribution

The demo code and original documentation use Apache-2.0, matching the Open
Knowledge repository. Linked Wikipedia pages remain under their own terms.
