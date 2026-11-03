#!/bin/bash
# Claude Code cloud-session bootstrap: submodules, Python tooling and the
# pinned Velociraptor binary, so `make conform` works from the first turn.
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "${CLAUDE_PROJECT_DIR}"
git submodule sync --quiet --recursive
git submodule update --init --recursive --depth 1
python3 -m pip install --quiet --ignore-installed --requirement .github/workflows/requirements/python.txt
scripts/velociraptor.sh > /dev/null
