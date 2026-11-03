#!/usr/bin/env bash
# Assemble one definitions directory for the pinned Velociraptor release:
# the upstream universe (.cache/definitions: Exchange, DetectRaptor, lint/)
# first, then every directory given, copied in order so that later
# directories override earlier ones (Dev wins over Prod).
#
#   scripts/definitions.sh OUT_DIR [DIR ...]     default DIRs: Prod Dev
#
# Velociraptor walks the directory in lexical order, so each source gets a
# numbered subdirectory. Needs .cache/definitions, which the Makefile builds.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

out="${1:?usage: scripts/definitions.sh OUT_DIR [DIR ...]}"
shift
dirs=("$@")
if [ "${#dirs[@]}" -eq 0 ]; then
  dirs=(Prod Dev)
fi

upstream_defs=".cache/definitions"
if [ ! -d "${upstream_defs}" ]; then
  echo "definitions.sh: ${upstream_defs} is missing; run a make target so the Makefile builds it" >&2
  exit 2
fi

copy_defs() {
  (cd "$1" && find . -type f \( -name '*.yaml' -o -name '*.yml' \) -print0) \
    | while IFS= read -r -d '' f; do
        mkdir -p "$2/$(dirname "${f}")"
        cp "$1/${f}" "$2/${f}"
      done
}

rm -rf "${out}"
mkdir -p "${out}"
copy_defs "${upstream_defs}" "${out}/0-upstream"
n=1
for d in "${dirs[@]}"; do
  if [ -d "${d}" ]; then
    copy_defs "${d}" "${out}/${n}-$(basename "${d}" | tr 'A-Z' 'a-z')"
  fi
  n=$((n + 1))
done
