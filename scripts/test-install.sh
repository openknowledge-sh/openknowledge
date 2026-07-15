#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) echo "installer tests require macOS or Linux" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) echo "installer tests require amd64 or arm64" >&2; exit 1 ;;
esac
asset="openknowledge_${os}_${arch}.tar.gz"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

write_binary() {
  path="$1"
  reported_version="$2"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'if [ "${1:-}" = "version" ]; then' \
    "  echo '$reported_version'" \
    '  exit 0' \
    'fi' \
    'exit 1' > "$path"
  chmod 0755 "$path"
}

write_release() {
  base="$1"
  reported_version="$2"
  payload="$base/payload"
  mkdir -p "$payload"
  write_binary "$payload/openknowledge" "$reported_version"
  tar -czf "$base/$asset" -C "$payload" openknowledge
  printf '%s  %s\n' "$(sha256_file "$base/$asset")" "$asset" > "$base/checksums.txt"
}

write_existing() {
  destination="$1"
  mkdir -p "$(dirname "$destination")"
  write_binary "$destination" "0.5.0"
}

run_installer() {
  base="$1"
  install_dir="$2"
  requested_version="$3"
  OPENKNOWLEDGE_BASE_URL="file://$base" \
    OPENKNOWLEDGE_VERSION="$requested_version" \
    OPENKNOWLEDGE_INSTALL_DIR="$install_dir" \
    bash "$root/install"
}

success_base="$tmp/success-release"
success_install="$tmp/success-bin"
mkdir -p "$success_base"
write_release "$success_base" "0.6.0"
write_existing "$success_install/openknowledge"
run_installer "$success_base" "$success_install" "v0.6.0" > "$tmp/success.log"
if [ "$("$success_install/openknowledge" version)" != "0.6.0" ]; then
  echo "installer test: successful install did not replace the old version" >&2
  exit 1
fi

checksum_base="$tmp/checksum-release"
checksum_install="$tmp/checksum-bin"
mkdir -p "$checksum_base"
write_release "$checksum_base" "0.6.0"
write_existing "$checksum_install/openknowledge"
printf 'corrupt' >> "$checksum_base/$asset"
if run_installer "$checksum_base" "$checksum_install" "0.6.0" > "$tmp/checksum.log" 2>&1; then
  echo "installer test: checksum mismatch unexpectedly succeeded" >&2
  exit 1
fi
if [ "$("$checksum_install/openknowledge" version)" != "0.5.0" ]; then
  echo "installer test: checksum failure replaced the existing binary" >&2
  exit 1
fi

syntax_base="$tmp/checksum-syntax-release"
syntax_install="$tmp/checksum-syntax-bin"
mkdir -p "$syntax_base"
write_release "$syntax_base" "0.6.0"
printf 'not-a-sha256  %s\n' "$asset" > "$syntax_base/checksums.txt"
write_existing "$syntax_install/openknowledge"
if run_installer "$syntax_base" "$syntax_install" "0.6.0" > "$tmp/checksum-syntax.log" 2>&1; then
  echo "installer test: malformed checksum unexpectedly succeeded" >&2
  exit 1
fi
if [ "$("$syntax_install/openknowledge" version)" != "0.5.0" ]; then
  echo "installer test: malformed checksum replaced the existing binary" >&2
  exit 1
fi

version_base="$tmp/version-release"
version_install="$tmp/version-bin"
mkdir -p "$version_base"
write_release "$version_base" "9.9.9"
write_existing "$version_install/openknowledge"
if run_installer "$version_base" "$version_install" "0.6.0" > "$tmp/version.log" 2>&1; then
  echo "installer test: version mismatch unexpectedly succeeded" >&2
  exit 1
fi
if [ "$("$version_install/openknowledge" version)" != "0.5.0" ]; then
  echo "installer test: version mismatch replaced the existing binary" >&2
  exit 1
fi
if find "$version_install" -maxdepth 1 -name '.openknowledge.install.*' -print -quit | grep -q .; then
  echo "installer test: version mismatch left a staging file" >&2
  exit 1
fi

missing_base="$tmp/missing-release"
missing_install="$tmp/missing-bin"
mkdir -p "$missing_base/payload"
printf 'missing binary\n' > "$missing_base/payload/README.md"
tar -czf "$missing_base/$asset" -C "$missing_base/payload" README.md
printf '%s  %s\n' "$(sha256_file "$missing_base/$asset")" "$asset" > "$missing_base/checksums.txt"
write_existing "$missing_install/openknowledge"
if run_installer "$missing_base" "$missing_install" "0.6.0" > "$tmp/missing.log" 2>&1; then
  echo "installer test: archive without binary unexpectedly succeeded" >&2
  exit 1
fi
if [ "$("$missing_install/openknowledge" version)" != "0.5.0" ]; then
  echo "installer test: missing binary replaced the existing binary" >&2
  exit 1
fi

directory_base="$tmp/directory-release"
directory_install="$tmp/directory-bin"
mkdir -p "$directory_base" "$directory_install/openknowledge"
write_release "$directory_base" "0.6.0"
if run_installer "$directory_base" "$directory_install" "0.6.0" > "$tmp/directory.log" 2>&1; then
  echo "installer test: directory destination unexpectedly succeeded" >&2
  exit 1
fi
if find "$directory_install" -maxdepth 1 -name '.openknowledge.install.*' -print -quit | grep -q .; then
  echo "installer test: failed install left a staging file" >&2
  exit 1
fi

invalid_install="$tmp/invalid-version-bin"
write_existing "$invalid_install/openknowledge"
if run_installer "$success_base" "$invalid_install" "../../malicious" > "$tmp/invalid-version.log" 2>&1; then
  echo "installer test: invalid requested version unexpectedly succeeded" >&2
  exit 1
fi
if [ "$("$invalid_install/openknowledge" version)" != "0.5.0" ]; then
  echo "installer test: invalid requested version replaced the existing binary" >&2
  exit 1
fi

echo "Shell installer transaction tests passed"
