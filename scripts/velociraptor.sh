#!/usr/bin/env bash
# Ensure the pinned Velociraptor release binary is present, then run it.
#
#   scripts/velociraptor.sh                 -> prints the binary path
#   scripts/velociraptor.sh artifacts verify Dev Prod
#
# The version pin lives in exactly one place:
# .github/workflows/requirements/velociraptor.txt (raised by scripts/upgrade.sh).
# Binaries are cached under .cache/velociraptor/ (gitignored); CI caches the
# same directory keyed on the pin file.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pin_file="${repo_root}/.github/workflows/requirements/velociraptor.txt"
version="$(tr -d '[:space:]' < "${pin_file}")"
cache_dir="${VELOCIRAPTOR_CACHE:-${repo_root}/.cache/velociraptor}"

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) asset="velociraptor-v${version}-linux-amd64-musl" ;;
  Linux-aarch64 | Linux-arm64) asset="velociraptor-v${version}-linux-arm64-musl" ;;
  Darwin-x86_64) asset="velociraptor-v${version}-darwin-amd64" ;;
  Darwin-arm64) asset="velociraptor-v${version}-darwin-arm64" ;;
  *)
    echo "velociraptor.sh: unsupported platform $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

binary="${cache_dir}/${asset}"
if [ ! -x "${binary}" ]; then
  mkdir -p "${cache_dir}"
  url="https://github.com/Velocidex/velociraptor/releases/download/v${version}/${asset}.gz"
  echo "velociraptor.sh: downloading ${url}" >&2
  tmp="$(mktemp "${cache_dir}/download.XXXXXX")"
  curl --fail --silent --show-error --location --retry 4 --retry-delay 2 \
    --output "${tmp}" "${url}"
  gunzip --stdout "${tmp}" > "${binary}.part"
  rm -f "${tmp}"
  chmod +x "${binary}.part"
  mv "${binary}.part" "${binary}"
fi

if [ "$#" -eq 0 ]; then
  echo "${binary}"
  exit 0
fi
exec "${binary}" "$@"
