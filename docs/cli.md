# CLI operations

This page keeps operational details out of the root README so the project
landing page can stay product-oriented.

## Install details

The shell installer downloads release assets from
`openknowledge-sh/openknowledge` GitHub Releases and verifies them against
`checksums.txt`.

`https://openknowledge.sh/install` serves this repository's `install` script
from the GitHub Pages site. The Pages build copies the root `install` file into
the published site, so the stable endpoint installs the latest GitHub Release by
default. The GitHub Release copy of the same installer is also available at:

```text
https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install
```

The web deployment workflow publishes `packages/web/dist` to GitHub Pages when
run manually from GitHub Actions. The artifact includes:

- the static site
- `CNAME` for `openknowledge.sh`
- `.nojekyll`
- `/install`

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
