#!/bin/sh

set -eu

repository="sirsjg/momentum"

fail() {
  printf 'momentum installer: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_command awk
require_command curl
require_command install
require_command mktemp
require_command tar
require_command uname

if [ "${MOMENTUM_INSTALL_DIR+x}" = "x" ]; then
  install_dir=$MOMENTUM_INSTALL_DIR
else
  [ -n "${HOME:-}" ] || fail 'HOME is not set; set MOMENTUM_INSTALL_DIR explicitly'
  install_dir=$HOME/.local/bin
fi
[ -n "$install_dir" ] || fail 'MOMENTUM_INSTALL_DIR must not be empty'

version=${MOMENTUM_VERSION:-}
if [ -z "$version" ]; then
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repository/releases/latest")
  version=${latest_url##*/}
fi
version=${version#v}
case "$version" in
  '' | *[!0-9A-Za-z.-]*) fail "invalid release version: $version" ;;
esac

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

archive="momentum_${version}_${os}_${arch}.tar.gz"
release_url="https://github.com/$repository/releases/download/v${version}"
temporary_dir=$(mktemp -d 2>/dev/null || mktemp -d -t momentum)

cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup 0
trap 'exit 1' HUP INT TERM

printf 'Downloading Momentum %s for %s/%s...\n' "$version" "$os" "$arch"
curl -fsSL "$release_url/$archive" -o "$temporary_dir/$archive"
curl -fsSL "$release_url/checksums.txt" -o "$temporary_dir/checksums.txt"

expected_checksum=$(
  awk -v archive="$archive" '$2 == archive { print $1; exit }' \
    "$temporary_dir/checksums.txt"
)
[ -n "$expected_checksum" ] || fail "checksum not found for $archive"

if command -v sha256sum >/dev/null 2>&1; then
  actual_checksum=$(sha256sum "$temporary_dir/$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  actual_checksum=$(shasum -a 256 "$temporary_dir/$archive" | awk '{ print $1 }')
else
  fail 'sha256sum or shasum is required to verify the download'
fi

[ "$actual_checksum" = "$expected_checksum" ] || fail "checksum verification failed for $archive"

tar -xzf "$temporary_dir/$archive" -C "$temporary_dir" momentum
mkdir -p "$install_dir"
install -m 0755 "$temporary_dir/momentum" "$install_dir/momentum"

printf 'Installed Momentum %s to %s/momentum\n' "$version" "$install_dir"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *)
    printf 'Add Momentum to your PATH with:\n  export PATH="%s:$PATH"\n' "$install_dir"
    ;;
esac
