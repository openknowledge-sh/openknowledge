#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
demo_tmp=$(mktemp -d)
trap 'rm -rf "$demo_tmp"' EXIT

openknowledge_bin=${OPENKNOWLEDGE_BIN:-}
if [[ -z "$openknowledge_bin" ]]; then
  openknowledge_bin="$demo_tmp/openknowledge"
  go build -o "$openknowledge_bin" "$repo_root/packages/cli/cmd/openknowledge"
fi

demo_names=(
  example-project-docs
  example-changelog
  example-research-notes
)

registry_file="$demo_tmp/registry.json"

for demo_name in "${demo_names[@]}"; do
  demo_root="$repo_root/examples/$demo_name"
  wiki_root="$demo_root/Wiki"
  site_root="$demo_tmp/$demo_name-site"

  "$openknowledge_bin" validate "$wiki_root"

  case "$demo_name" in
    example-project-docs)
      query="what context affects citation validation?"
      rules="project,docs,decisions"
      ;;
    example-changelog)
      query="what changed for invalid source URLs?"
      rules="docs,changelog"
      ;;
    example-research-notes)
      query="what evidence supports the citation workflow?"
      rules="research"
      ;;
  esac

  "$openknowledge_bin" search "$wiki_root" "$query" --matches --limit 3 >/dev/null
  "$openknowledge_bin" prompt rules apply "$rules" \
    --path "$wiki_root" \
    --file "$demo_root/AGENTS.md" \
    --dry-run >/dev/null
  "$openknowledge_bin" export html --out "$site_root" "$wiki_root"

  test -f "$site_root/index.html"
  test -f "$site_root/openknowledge.json"
  test -f "$site_root/assets/openknowledge-bundle.tar.gz"

  OPENKNOWLEDGE_REGISTRY_FILE="$registry_file" \
    "$openknowledge_bin" connect "$wiki_root" --as "$demo_name" >/dev/null
  OPENKNOWLEDGE_REGISTRY_FILE="$registry_file" \
    "$openknowledge_bin" validate --quiet "$demo_name"
  OPENKNOWLEDGE_REGISTRY_FILE="$registry_file" \
    "$openknowledge_bin" search "$demo_name" "$query" --matches --limit 3 >/dev/null

  if [[ -f "$demo_root/package.json" ]]; then
    (
      cd "$demo_root"
      node --test
    )
  fi
done
