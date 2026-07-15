# @openknowledge-sh/openknowledge

NPM wrapper for the `openknowledge` CLI, a small tool for creating LLM wiki
tooling and LLM Wikipedia-style Markdown knowledge bases for agents.

```sh
npm install -g @openknowledge-sh/openknowledge
openknowledge version
```

The package downloads the matching binary from GitHub Releases during
installation. Set `OPENKNOWLEDGE_VERSION=latest` to install the latest GitHub
release instead of the npm package version.

The postinstall downloader requires credential-free HTTPS across a maximum of
five redirects, applies finite response and decompression limits, verifies one
exact SHA-256 entry, and accepts only one exact regular `openknowledge` member
from the release tarball. The verified binary is staged beside its destination
and renamed into place atomically.

Published package versions match the GitHub release tag without its leading
`v`. The release workflow verifies this invariant before creating the tag and
publishes the wrapper with npm provenance after the binary release succeeds.
Each checksummed platform archive also receives GitHub/Sigstore build
provenance that can be verified with `gh attestation verify <archive> -R
openknowledge-sh/openknowledge`.

Open Knowledge bundles follow OKF v0.1: Markdown with YAML frontmatter that is
easy to inspect with shell tools and coding agents.

This package includes `THIRD_PARTY_NOTICES.md` and the upstream Apache-2.0
license copy for the embedded OKF spec.
