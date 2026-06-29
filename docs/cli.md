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

## Web deploy head injection

`pnpm build:web` can inject trusted HTML into the generated site `<head>`.
Use this for analytics, verification meta tags, or small loader scripts without
hard-coding a provider into the repository.

- `OPENKNOWLEDGE_HEAD_FILE=./head.html` inserts an HTML fragment file.
- `OPENKNOWLEDGE_HEAD_HTML='<meta name="..." content="...">'` inserts an inline
  HTML fragment.
- `OPENKNOWLEDGE_SCRIPT_SRC=/analytics.js` inserts one or more script tags. Use
  comma or newline separation for multiple sources.

For Google Analytics, put the provider snippet in `head.html` and build with:

```sh
OPENKNOWLEDGE_HEAD_FILE=./head.html pnpm build:web
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

The prompt asks the agent to interview the user, choose where the knowledge base
should live, run `openknowledge new --name "<name>" "<path>"`, read the
generated setup files, and turn the generic scaffold into an agentic wiki.

During setup the agent should create or update:

- `AGENTS.md` with local rules for when future agents should read and update the
  wiki
- `workflows/` with repeatable maintenance behaviors such as docs updates,
  changelog updates, feature memory, bug triage, or research import
- `skills/` with local agent-tool guidance for using `openknowledge list`,
  reading relevant pages, applying workflows, and validating changes
- `automations/` with specs for recurring or external jobs when the user wants
  them

The agent should run `openknowledge validate "<path>"`, fix issues, and delete
`SETUP.MD` only after setup is complete.

## Local viewer

`openknowledge open [path]` starts a local HTTP viewer for a knowledge base and
prints the URL:

```sh
openknowledge open ./project-memory
```

By default it binds to `127.0.0.1` on a free port and keeps running until the
process is stopped. Use `--host` or `--port` when a fixed address is needed.

The viewer renders Markdown files, strips YAML frontmatter from document pages,
rewrites relative Markdown links between `.md` files, and shows inline
validation issues from the bundle listing.

The viewer can also inject trusted custom `<head>` HTML, matching the web deploy
contract:

```sh
openknowledge open --head-file ./head.html ./project-memory
openknowledge open --script-src /analytics.js ./project-memory
```

The same values can be supplied with `OPENKNOWLEDGE_HEAD_FILE`,
`OPENKNOWLEDGE_HEAD_HTML`, and comma- or newline-separated
`OPENKNOWLEDGE_SCRIPT_SRC`.

`openknowledge validate` reports broken local Markdown links as warnings. It
does not fail the bundle for link warnings because OKF v0.1 keeps link targets
outside the required conformance rules.

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
