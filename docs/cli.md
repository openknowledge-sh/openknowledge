# CLI operations

This page keeps operational details out of the root README so the project
landing page can stay product-oriented.

## Install details

The shell installer downloads release assets from
`openknowledge-sh/openknowledge` GitHub Releases and verifies them against
`checksums.txt`.

`https://openknowledge.sh/install` should serve this repository's `install`
script. The simplest deployment is a redirect to the latest GitHub Release
asset:

```text
https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install
```

The npm package downloads the matching binary from GitHub Releases during
installation. Set `OPENKNOWLEDGE_VERSION=latest` to install the latest GitHub
release instead of the npm package version.

## Workspace

```text
packages/cli  - Go CLI
packages/npm  - npm wrapper for the release binary
packages/web  - static HTML/CSS site
```

## Develop

```sh
pnpm test:cli
pnpm build:cli
pnpm build:web
pnpm dev:web
```

## Setup prompt

`openknowledge setup` prints only an agent prompt to stdout. With interactive
Codex, pass the prompt as an argument so stdin stays attached to the terminal:

```sh
codex "$(openknowledge setup)"
```

Do not use `openknowledge setup | codex` with interactive Codex; Codex will
exit with `stdin is not a terminal`. A pipe is only appropriate for an agent CLI
that explicitly accepts prompts from stdin.

The prompt asks the agent to inspect the current workspace or target folder,
read relevant user or project memories when the runtime exposes them, ask only
the missing setup questions, choose where the knowledge base should live, run
`openknowledge new --name "<name>" "<path>"`, read the generated setup files,
and turn the minimal scaffold into an agentic wiki with only the folders that
fit the discovered context and user's answers.

During setup the agent should create or update:

- `AGENTS.md` with local rules for when future agents should read and update the
  wiki
- workflow docs for selected repeatable maintenance behaviors such as docs
  updates, changelog updates, feature memory, bug triage, or research import
- repo-scoped or user-scoped agent instructions when local agent-tool behavior
  should be reusable, such as using `openknowledge list`, reading relevant
  pages, applying workflows, and validating changes
- wiki skill pages only when they are useful as documentation or references, not
  as the default place where executable agent skills live
- native automations in Codex app, Cowork, or another available orchestrator
  only when the runtime can create them and the user approves
- automation candidate notes or manual workflows when recurring behavior would
  help but no native automation is available or approved
- any domain-specific folders and seed pages needed for the selected use case

The agent should run `openknowledge validate "<path>"`, fix issues, and delete
`SETUP.MD` only after setup is complete.

## Local viewer

`openknowledge open` starts a local HTTP viewer from the registry and prints the
URL:

```sh
openknowledge open
openknowledge open ./project-memory
```

With no path, the viewer shows registered knowledge bases in a left workspace
selector. If the registry contains one knowledge base, that one is selected. If
the registry contains several, choose one from the selector. `openknowledge open
<path-or-name>` opens that folder or registry alias directly.

By default it binds to `127.0.0.1` on a free port and keeps running until the
process is stopped. Use `--host` or `--port` when a fixed address is needed.
Custom local names such as `openknowledge.local` need to resolve to the local
machine through `/etc/hosts`, local DNS, or a reverse proxy; the CLI does not
create hostname aliases itself.

The viewer renders Markdown files, strips YAML frontmatter from document pages,
rewrites relative Markdown links between `.md` files, and shows inline
validation issues from the bundle listing. The index page includes local
full-text search across paths, titles, metadata, headings, and document bodies,
with light fuzzy and diacritic-insensitive matching.

`openknowledge validate` reports broken local Markdown links as warnings. It
does not fail the bundle for link warnings because OKF v0.1 keeps link targets
outside the required conformance rules.

Validation also warns for non-blocking Markdown syntax problems, such as
malformed links or unclosed code fences, and for frontmatter formatting issues
that can still be parsed. Frontmatter that cannot be parsed is an error because
the validator cannot safely apply OKF frontmatter rules after that point.

## Registry

The registry stores named local paths for shared or standalone knowledge bases:

```sh
openknowledge registry add personal ~/knowledge
openknowledge registry list
openknowledge where personal
```

Registry names are only aliases for filesystem paths. Commands that read a
bundle accept either form:

```sh
openknowledge open personal
openknowledge list personal
openknowledge list ~/knowledge
openknowledge validate personal
```

The registry is also the default source for `openknowledge open` when no path is
provided.

For agent workflows, prefer `openknowledge where <name>` to discover the actual
folder, then use normal filesystem tools such as `rg`, file reads, and edits
against that path. The CLI registry does not replace direct navigation of the
Markdown bundle.

## Static exports

`openknowledge to html [path] --out <folder>` writes one `.html` file for each
Markdown file in the bundle. It strips YAML frontmatter from rendered pages and
rewrites Markdown links between local `.md` files to their generated `.html`
targets.

```sh
openknowledge to html ./project-memory --out ./project-site
```

`openknowledge to json [path]` writes the normalized bundle model as JSON. The
JSON includes file metadata, frontmatter scalar values, Markdown body content,
local and external links, and validation issues. It prints to stdout by default
or writes to `--out <file>`.

```sh
openknowledge to json ./project-memory
openknowledge to json ./project-memory --out ./bundle.json
```

## Release

GitHub Releases are the source of truth for downloadable binaries. Run the
release manually from GitHub Actions:

```text
Actions -> Release -> Run workflow -> version: 0.1.0
```

The workflow normalizes the input to a `v*` tag, creates and pushes that tag,
then runs GoReleaser. GoReleaser uploads the installer, checksums, license
files, third-party notices, and platform archives to GitHub Releases. The
workflow fails before creating a release if the version is malformed or if an
existing tag points at a different commit. A rerun may reuse an existing tag
when it already points at the current workflow commit.

The npm publishing job is present in the workflow as commented YAML, but it is
disabled for now while the GitHub Release artifact flow is validated first. When
re-enabling npm publishing:

- set `packages/npm/package.json` `version` to match the tag without the leading
  `v`
- configure the repository `NPM_TOKEN` secret with permission to publish
  `@openknowledge-sh/openknowledge`

The commented npm publish job is designed to fail fast if the package version
does not match the tag or if `NPM_TOKEN` is missing.

Local installer test against a directory of release assets:

```sh
OPENKNOWLEDGE_BASE_URL=file:///tmp/openknowledge-release \
OPENKNOWLEDGE_INSTALL_DIR=/tmp/openknowledge-bin \
bash install
```

Manual npm publish fallback after the matching GitHub Release exists:

```sh
cd packages/npm
npm publish --access public
```
