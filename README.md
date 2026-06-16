# Open Knowledge CLI

Open Knowledge is a small CLI for creating, validating, and inspecting local
Open Knowledge Format bundles.

It is built for teams that want project knowledge to stay portable, readable in
Git, and easy for both humans and coding agents to navigate.

## Start in 30 seconds

Install with the shell installer:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

Or install the npm wrapper:

```sh
npm install -g @openknowledge-sh/openknowledge
```

Create and inspect a new knowledge bundle:

```sh
openknowledge new ./project-memory
openknowledge list ./project-memory
openknowledge validate ./project-memory
```

## Why Open Knowledge

- **Portable by default**: knowledge lives in Markdown files with predictable
  names, frontmatter, indexes, and logs.
- **Agent-readable**: new bundles include `AGENTS.md`, `SETUP.MD`, and a local
  pinned `SPEC.md` so an agent can pick up the setup flow without hidden state.
- **Spec-backed**: validation targets an embedded Open Knowledge Format spec
  version, starting with OKF v0.1.

## How it works

`openknowledge new` creates a local bundle with the base OKF structure, a setup
handoff, agent guidance, an update log, and a pinned copy of the current spec.

After that, humans and agents edit normal Markdown files. `openknowledge
validate` checks the bundle for portable OKF structure, and `openknowledge list`
prints the bundle tree with inline validation issues.

The intended loop is:

```text
new bundle -> agent setup -> edit knowledge -> list/validate -> commit
```

## Commands

| Command | Purpose |
| --- | --- |
| `openknowledge new [folder]` | Scaffold a local Open Knowledge bundle. |
| `openknowledge spec latest` | Print the latest embedded OKF spec. |
| `openknowledge spec 0.1` | Print a specific embedded spec version. |
| `openknowledge validate [path]` | Validate a bundle against the latest spec. |
| `openknowledge validate --spec 0.1 [path]` | Validate against a specific spec version. |
| `openknowledge list [path]` | Print a bundle tree with inline validation issues. |
| `openknowledge list --spec 0.1 [path]` | List while validating against a specific spec version. |
| `openknowledge list -json [path]` | Print machine-readable inventory output. |
| `openknowledge version` | Print the CLI version. |

## What validation checks

The validator enforces the OKF v0.1 rules that matter for a portable bundle:

- every non-reserved Markdown file has top-level YAML frontmatter
- every concept frontmatter has a non-empty `type`
- `index.md` and `log.md` are reserved files, not concept documents
- root `index.md` may declare `okf_version: "0.1"`
- `log.md` `##` headings use `YYYY-MM-DD`

It does not fail on optional fields, unknown concept types, unknown frontmatter
keys, broken links, or missing index files.

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

## Release

GitHub Releases are the source of truth for downloadable binaries:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The tag starts the GitHub Actions release workflow, which runs GoReleaser and
uploads the installer, checksums, license files, third-party notices, and
platform archives.

Local installer test against a directory of release assets:

```sh
OPENKNOWLEDGE_BASE_URL=file:///tmp/openknowledge-release \
OPENKNOWLEDGE_INSTALL_DIR=/tmp/openknowledge-bin \
bash install
```

Publish the npm wrapper from `packages/npm/` after the matching GitHub Release
exists:

```sh
cd packages/npm
npm publish --access public
```

## License and attribution

Open Knowledge is licensed under Apache-2.0.

The embedded OKF spec copy is Apache-2.0 material from
`GoogleCloudPlatform/knowledge-catalog`. See `THIRD_PARTY_NOTICES.md` and
`packages/cli/internal/okf/assets/specs/README.md` for attribution and license
handling.
