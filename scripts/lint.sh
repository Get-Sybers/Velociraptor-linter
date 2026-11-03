#!/usr/bin/env bash
# The artefact standard as VQL (lint/Artefacts.yaml), run by the pinned
# Velociraptor release. Prints one line per finding, `path:line: RULE
# severity: message`, then a summary, and exits 1 when any error was
# reported.
#
#   scripts/lint.sh [--strict] [--root DIR] [path ...]       default: Dev Prod
#   scripts/lint.sh --root tests/lint --expect FILE ...       compare with FILE
#   scripts/lint.sh --root tests/lint --update-expected FILE  rewrite FILE
#
# Warnings are errors under --strict and always for files under Prod/.
# Paths are relative to the root (default: the repository). Needs
# .cache/definitions, which the Makefile builds: the Exchange, DetectRaptor
# and lint/ definitions the linter resolves against.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

strict=FALSE
root="${repo_root}"
expect=""
update=""
paths=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --strict) strict=TRUE ;;
    --root) root="$(cd "$2" && pwd)"; shift ;;
    --expect) expect="$2"; shift ;;
    --update-expected) update="$2"; shift ;;
    *) paths+=("${1#./}") ;;
  esac
  shift
done
if [ "${#paths[@]}" -eq 0 ]; then
  paths=(Dev Prod)
fi
json="["
sep=""
for p in "${paths[@]}"; do
  json="${json}${sep}\"${p}\""
  sep=","
done
json="${json}]"

defs=".cache/definitions"
if [ ! -f "${defs}/lint/Artefacts.yaml" ]; then
  echo "lint.sh: ${defs} lacks the lint definitions; run \`make vql-lint\` once" >&2
  exit 2
fi
# The pinned Velociraptor's VQL reference: the plugins and functions the
# improper-method rules (VS080 to VS082) check against.
reference="${repo_root}/upstream/velociraptor/docs/references/vql.yaml"
if [ ! -f "${reference}" ]; then
  echo "lint.sh: ${reference} is missing; run \`make submodules\`" >&2
  exit 2
fi
report=".cache/lint/report.txt"
mkdir -p "$(dirname "${report}")"
rm -f "${report}"

query="LET F <= SELECT format(format='%v:%v: %v %v: %v', args=[Path, Line, Rule, Severity, Message]) AS T
         FROM Artifact.Linter.Artefacts(Root='${root}', Paths=${json}, Strict=${strict}, Reference='${reference}')
       SELECT copy(filename=if(condition=len(list=F.T) > 0, then=join(array=F.T, sep='\\n') + '\\n', else=''),
                   accessor='data', dest='${repo_root}/${report}') AS Written
       FROM scope()"
scripts/velociraptor.sh --definitions "${defs}" query --format jsonl "${query}" > /dev/null
sort -o "${report}" "${report}"

if [ -n "${update}" ]; then
  cp "${report}" "${update}"
  echo "lint.sh: wrote $(wc -l < "${update}" | tr -d ' ') expected findings to ${update}"
  exit 0
fi
if [ -n "${expect}" ]; then
  if diff -u "${expect}" "${report}"; then
    echo "lint-test: $(wc -l < "${report}" | tr -d ' ') findings as expected"
    exit 0
  fi
  echo "lint-test: findings differ from ${expect} (run \`make lint-test-update\` if the change is intended)" >&2
  exit 1
fi

errors=$(grep -c ' error: ' "${report}" || true)
warnings=$(grep -c ' warning: ' "${report}" || true)
if [ "${GITHUB_ACTIONS:-}" = "true" ]; then
  sed -E 's/^([^:]+):([0-9]+): (VS[0-9]+) (error|warning): (.*)$/::\4 file=\1,line=\2,title=\3::\5/' "${report}"
else
  cat "${report}"
fi
echo "vql-lint: ${errors} errors, ${warnings} warnings"
[ "${errors}" -eq 0 ]
