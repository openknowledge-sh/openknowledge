# Open Knowledge CLI

CLI tool for managing Open Knowledge Format (OKF) bundles.

## Install

Shell installer:

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

NPM:

```sh
npm install -g @openknowledge-sh/openknowledge
openknowledge version
```

The shell installer expects `https://openknowledge.sh/install` to serve this
repository's `install` script. The simplest setup is a redirect from
`https://openknowledge.sh/install` to the latest GitHub Release asset:

```text
https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install
```

The installer downloads release assets from `openknowledge-sh/openknowledge`
GitHub Releases and verifies them against `checksums.txt`.

## Commands

```sh
openknowledge new [folder]
openknowledge spec latest
openknowledge spec 0.1
openknowledge validate [path]
openknowledge validate --spec 0.1 [path]
openknowledge list [path]
openknowledge list --spec 0.1 [path]
openknowledge list -json [path]
openknowledge version
```

## Setup Flow

```sh
openknowledge new
```

The wizard asks for a knowledge base name, creates a folder with the base OKF
structure, seeds lightweight `AGENTS.md`, writes `SETUP.MD`, stores a local
`SPEC.md`, and prints an agent handoff prompt. The agent should read
`SETUP.MD`, inspect the scaffold, interview the user, update `AGENTS.md`, then
create the initial local Open Knowledge wiki, rules, indexes, and seed pages.
After successful setup, the agent should validate the bundle and delete
`SETUP.MD`.

`openknowledge spec latest` prints the latest embedded OKF spec. Specs are
versioned internally under `internal/okf/assets/specs/`; new knowledge bases
store the current latest content as a local `SPEC.md` concept.

`openknowledge validate` defaults to the latest embedded spec. Use
`openknowledge validate --spec 0.1 <path>` to validate against a specific
version.

`openknowledge list` prints a bundle tree and annotates entries with inline OKF
validation issues. Reserved `index.md` and `log.md` files are included but
rendered in a muted color. Files with issues are rendered in red with a short
message. Use `openknowledge list --spec 0.1` to check against a specific spec
version and `openknowledge list -json` for machine-readable inventory output.

## What `validate` checks

The validator enforces the OKF v0.1 conformance rules that matter for a
portable bundle:

- every non-reserved `.md` file has top-level YAML frontmatter
- every concept frontmatter has a non-empty `type`
- `index.md` and `log.md` are reserved files, not concept documents
- root `index.md` may declare `okf_version: "0.1"`
- `log.md` `##` headings use `YYYY-MM-DD`

It does not fail on missing optional fields, unknown concept types, unknown
frontmatter keys, broken links, or missing index files.

## Build

```sh
go build ./cmd/openknowledge
```

## Release

GitHub Releases are the source of truth for downloadable binaries:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The tag starts the GitHub Actions release workflow, which runs GoReleaser.

The release uploads:

- `install`
- `checksums.txt`
- `openknowledge_darwin_amd64.tar.gz`
- `openknowledge_darwin_arm64.tar.gz`
- `openknowledge_linux_amd64.tar.gz`
- `openknowledge_linux_arm64.tar.gz`
- `openknowledge_windows_amd64.tar.gz`
- `openknowledge_windows_arm64.tar.gz`

Local installer test against a directory of release assets:

```sh
OPENKNOWLEDGE_BASE_URL=file:///tmp/openknowledge-release \
OPENKNOWLEDGE_INSTALL_DIR=/tmp/openknowledge-bin \
bash install
```

Publish the npm wrapper from `npm/` after the matching GitHub Release exists:

```sh
cd npm
npm publish --access public
```
