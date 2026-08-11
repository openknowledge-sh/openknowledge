# @openknowledge-sh/openknowledge

NPM wrapper for the `openknowledge` CLI.

Flexible knowledge bases in Markdown that your agents can create, retrieve,
validate, and publish.

Node 20 or a later version is required.

```sh
npm install -g @openknowledge-sh/openknowledge
openknowledge version
# Short alias:
okn version
```

The `openknowledge` and `okn` commands run the same installed CLI.

Before the CLI sends its first event, it discloses default-on anonymous usage
and sanitized error telemetry. Run `okn telemetry show-payload` to inspect a
sample. Use `okn --no-telemetry <command>` or `okn telemetry disable` to save an
opt-out. Telemetry does not include command arguments, paths, content,
identities, output, hostnames, IP addresses, or raw user agents.

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

Open Knowledge bundles follow OKF v0.2: Markdown with YAML frontmatter that is
easy to inspect with shell tools and coding agents.

This package includes `THIRD_PARTY_NOTICES.md` and the upstream Apache-2.0
license copy for the embedded OKF spec.
