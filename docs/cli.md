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

## Release

GitHub Releases are the source of truth for downloadable binaries:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The tag starts the GitHub Actions release workflow. GoReleaser uploads the
installer, checksums, license files, third-party notices, and platform archives
to GitHub Releases.

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
