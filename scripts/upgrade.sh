#!/usr/bin/env bash
# Raise every pin this repository owns to its ceiling, then run the gate.
#   - submodules: each moves to the tip of its tracked branch
#   - Velociraptor binary: the highest vX.Y.Z release tag
# Python pins and GitHub Actions are raised by Dependabot (.github/dependabot.yml).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

git submodule sync --quiet --recursive
git submodule update --remote --recursive --depth 1

pin_file=".github/workflows/requirements/velociraptor.txt"
latest="$(git ls-remote --tags --refs https://github.com/Velocidex/velociraptor.git \
  | awk -F/ '{print $NF}' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1)"
latest="${latest#v}"
if [ -n "${latest}" ] && [ "${latest}" != "$(tr -d '[:space:]' < "${pin_file}")" ]; then
  echo "velociraptor: $(cat "${pin_file}") -> ${latest}"
  printf '%s\n' "${latest}" > "${pin_file}"
fi

git status --short
make conform
