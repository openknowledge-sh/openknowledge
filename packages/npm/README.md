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

Open Knowledge bundles follow OKF v0.1: Markdown with YAML frontmatter that is
easy to inspect with shell tools and coding agents.

This package includes `THIRD_PARTY_NOTICES.md` and the upstream Apache-2.0
license copy for the embedded OKF spec.
