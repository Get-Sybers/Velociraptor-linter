#!/usr/bin/env bash
# Smoke: every client artefact runs once on every smoke host, through the
# pinned Velociraptor release, and must not log an error. The test knows
# nothing about any artefact: tests/smoke/harness.vql enumerates them with
# artifact_definitions() and collects each with collect(); the hosts are
# Velociraptor's own dead-disk remapping (--remap, tests/smoke/<host>.yaml),
# impersonating an operating system over a directory filled with the
# sample files Velociraptor ships (tests/smoke/files.txt); the verdict is
# Velociraptor's own: a log line at level ERROR, or one matching the
# regexes its flow runner and golden runner call fatal.
#
#   scripts/smoke.sh [--strict] [--host windows|linux|darwin]... [--filter REGEX] [--verbose]
#   scripts/smoke.sh --root DIR --filter REGEX --expect FILE          compare with FILE
#   scripts/smoke.sh --root DIR --filter REGEX --update-expected FILE  rewrite FILE
#
# --root names the definition directories to load (default: Prod Dev);
# --filter the artefact names to run (default: every artefact defined under the
# roots, so all client artefacts present are covered). Each artefact
# ends as one of: ran, skipped (no source's precondition held on that
# host), skipped-tool (it declares a tool no inventory can serve here) or
# FAILED. As with lint warnings, a failure is reported for an artefact in
# Dev/ and fails the run for one in Prod/, or for any under --strict.
# Needs .cache/definitions, which the Makefile builds, and the
# upstream/velociraptor submodule for the sample files.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

hosts=()
roots=()
filter=''
verbose=0
strict=0
expect=""
update=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --host) hosts+=("$2"); shift ;;
    --root) roots+=("$2"); shift ;;
    --filter) filter="$2"; shift ;;
    --verbose) verbose=1 ;;
    --strict) strict=1 ;;
    --expect) expect="$2"; shift ;;
    --update-expected) update="$2"; shift ;;
    *) echo "smoke.sh: unknown argument $1" >&2; exit 2 ;;
  esac
  shift
done
[ "${#hosts[@]}" -eq 0 ] && hosts=(windows linux darwin)
[ "${#roots[@]}" -eq 0 ] && roots=(Prod Dev)

# Default filter: the names defined under the roots, so smoke covers every
# client artefact the repository ships without naming any one of them. When
# the trees hold none, match nothing (there is nothing to smoke).
if [ -z "${filter}" ]; then
  names="$( for r in "${roots[@]}"; do [ -d "${r}" ] && grep -rh '^name:' "${r}" 2>/dev/null || true; done \
    | sed -E 's/^name:[[:space:]]*//; s/["'"'"']//g' | sort -u )"
  if [ -n "${names}" ]; then
    alt="$(printf '%s' "${names}" | paste -sd'|' -)"
    filter="^(${alt})$"
  else
    filter='^$'
  fi
fi

testdata="${repo_root}/upstream/velociraptor/artifacts/testdata/files"
if [ ! -f "${testdata}/ntuser.dat" ]; then
  echo "smoke.sh: ${testdata} is missing; run \`make submodules\`" >&2
  exit 2
fi

cache=".cache/smoke"
defs="${cache}/definitions"
scripts/definitions.sh "${defs}" "${roots[@]}"
harness="$(grep -v '^--' tests/smoke/harness.vql)"  # the CLI would read a leading VQL comment as a flag
report="${cache}/status.txt"
mkdir -p "${cache}/hosts"
: > "${report}"

# Build the host's directory from tests/smoke/files.txt (tab separated:
# host, path under the host root, sample file).
build_host() {
  local host="$1" root="$2"
  rm -rf "${root}"
  mkdir -p "${root}/C" "${root}/rootfs" "${root}/registry"
  grep -v '^#' tests/smoke/files.txt | awk -F'\t' -v h="${host}" '$1 == h' \
    | while IFS=$'\t' read -r _ dest src; do
        mkdir -p "${root}/$(dirname "${dest}")"
        cp "${testdata}/${src}" "${root}/${dest}"
      done
}

# Turn one host's log into status lines: host, status, artefact, detail.
# The harness brackets every artefact with `smoke: begin` and `smoke: end`;
# Velociraptor logs `Starting collection of <artefact>` once a source's
# precondition held. Fatal messages are Velociraptor's own lists
# (constants.VQL_ERROR_REGEX and the golden runner's), plus the sandbox's
# refusals, which mean the host is incomplete rather than the artefact.
classify() {
  local host="$1" log="$2"
  awk -v host="${host}" '
    function short(m) {
      sub(/ Current Scope is .*$/, "", m)
      sub(/[[:space:]]+$/, "", m)
      if (length(m) > 220) m = substr(m, 1, 217) "..."
      return m
    }
    function emit() {
      printf "%s\t%s\t%s\t%s\n", host, status, cur, detail
    }
    BEGIN {
      fatal = "error:|symbol .+ not found|expecting a path arg type, not|field .+ is required|unexpected arg|artifact .+ not found|order by column .+ not present in row|panic runtime error:|extra unrecognized arg|unknown filesystem accessor|permissiondenied|permission denied"
      cur = ""
    }
    {
      level = ""
      if (match($0, /"level":"[a-z]+"/)) level = substr($0, RSTART + 9, RLENGTH - 10)
      msg = ""
      if (match($0, /"msg":"/)) {
        msg = substr($0, RSTART + 7)
        i = index(msg, "\",\"time\":")
        if (i > 0) msg = substr(msg, 1, i - 1)
      }
      gsub(/\\n/, " ", msg)
      gsub(/\\"/, "\"", msg)
      gsub(/\\\\/, "\\", msg)
      if (index(msg, "smoke: begin ") == 1) { cur = substr(msg, 14); sub(/[[:space:]]+$/, "", cur); status = "skipped"; detail = ""; nmsg = 0; delete seen; next }
      if (index(msg, "smoke: end ") == 1) { emit(); cur = ""; next }
      bad = (level == "error" || (index(msg, "Loaded ") != 1 && tolower(msg) ~ fatal))
      if (cur == "") {
        if (bad) { printf "%s\tFAILED\t(harness)\t%s\n", host, short(msg) }
        next
      }
      if (index(msg, "Starting collection of " cur) == 1) { if (status == "skipped") status = "ran"; next }
      if (msg ~ /^collect: Tool .* has no url defined/) { status = "skipped-tool"; sub(/^collect: /, "", msg); detail = short(msg); next }
      if (bad) {
        status = "FAILED"
        m = short(msg)
        if (!(m in seen)) {
          seen[m] = 1
          nmsg++
          if (nmsg <= 5) detail = detail (detail == "" ? "" : " | ") m
          else if (nmsg == 6) detail = detail " | ..."
        }
      }
    }
    END { if (cur != "") { status = "FAILED"; detail = detail (detail == "" ? "" : " | ") "Velociraptor stopped before the artefact finished"; emit() } }
  ' "${log}"
}

for host in "${hosts[@]}"; do
  template="tests/smoke/${host}.yaml"
  if [ ! -f "${template}" ]; then
    echo "smoke.sh: no such host ${host} (tests/smoke/${host}.yaml)" >&2
    exit 2
  fi
  root="${repo_root}/${cache}/hosts/${host}"
  build_host "${host}" "${root}"
  remap="${cache}/hosts/${host}.yaml"
  sed -e "s#__ROOT__#${root}#g" -e "s#__TESTDATA__#${testdata}#g" "${template}" > "${remap}"
  log="${cache}/hosts/${host}.log"
  out="${cache}/hosts/${host}.jsonl"
  err="${cache}/hosts/${host}.err"
  rm -f "${log}"
  rc=0
  scripts/velociraptor.sh --remap "${remap}" --definitions "${defs}" \
    query --format jsonl --nobanner --logfile "${log}" --env "Filter=${filter}" "${harness}" \
    > "${out}" 2> "${err}" || rc=$?
  classify "${host}" "${log}" >> "${report}"
  if [ "${rc}" -ne 0 ] && ! grep -q "^${host}	FAILED	" "${report}"; then
    printf '%s\tFAILED\t(harness)\t%s\n' "${host}" "exit ${rc}: $(tail -n 1 "${err}" | cut -c1-220)" >> "${report}"
  fi
done
sort -o "${report}" "${report}"

if [ -n "${update}" ]; then
  cp "${report}" "${update}"
  echo "smoke.sh: wrote $(wc -l < "${update}" | tr -d ' ') status lines to ${update}"
  exit 0
fi
if [ -n "${expect}" ]; then
  if diff -u "${expect}" "${report}"; then
    echo "smoke-test: $(wc -l < "${report}" | tr -d ' ') status lines as expected"
    exit 0
  fi
  echo "smoke-test: status differs from ${expect} (run \`make smoke-test-update\` if the change is intended)" >&2
  exit 1
fi

# Report: every failure, each tool skip once, all lines under --verbose,
# then a summary per host and the artefacts no host ran. A failure counts
# against the run when the artefact is in Prod/ (by name), the harness
# itself failed, or --strict is set; otherwise it is reported.
prod_names=" $( { grep -rh '^name:' Prod 2>/dev/null || true; } | sed -E 's/^name:[[:space:]]*//; s/["'"'"']//g' | tr '\n' ' ') "
failed=0
reported=0
tools_seen=""
while IFS=$'\t' read -r host status artefact detail; do
  case "${status}" in
    FAILED)
      case "${prod_names}" in
        *" ${artefact} "*) counts=1 ;;
        *) counts="${strict}" ;;
      esac
      [ "${artefact}" = "(harness)" ] && counts=1
      if [ "${counts}" -eq 1 ]; then
        failed=$((failed + 1))
        level=error
        echo "FAILED  ${artefact} [${host}]: ${detail}"
      else
        reported=$((reported + 1))
        level=warning
        echo "failed  ${artefact} [${host}] (Dev, reported): ${detail}"
      fi
      if [ "${GITHUB_ACTIONS:-}" = "true" ]; then
        echo "::${level} title=smoke ${host}::${artefact}: ${detail}"
      fi ;;
    skipped-tool)
      case " ${tools_seen} " in
        *" ${artefact} "*) ;;
        *) tools_seen="${tools_seen} ${artefact}"; echo "skipped ${artefact}: ${detail}" ;;
      esac ;;
    *) [ "${verbose}" -eq 1 ] && echo "${status}  ${artefact} [${host}]" ;;
  esac
done < "${report}"
for host in "${hosts[@]}"; do
  printf 'smoke [%s]: %s ran, %s skipped (precondition), %s skipped (tool), %s failed\n' "${host}" \
    "$(grep -c "^${host}	ran	" "${report}" || true)" \
    "$(grep -c "^${host}	skipped	" "${report}" || true)" \
    "$(grep -c "^${host}	skipped-tool	" "${report}" || true)" \
    "$(grep -c "^${host}	FAILED	" "${report}" || true)"
done
never="$(awk -F'\t' '$3 != "(harness)" { seen[$3] = 1; if ($2 != "skipped") ran[$3] = 1 } END { for (a in seen) if (!(a in ran)) print a }' "${report}" | sort)"
if [ -n "${never}" ]; then
  echo "smoke: no host satisfied a precondition of: $(echo "${never}" | tr '\n' ' ')"
fi
if [ "${reported}" -gt 0 ]; then
  echo "smoke: ${reported} Dev failure(s) reported, not fatal (fatal in Prod/ or under --strict)"
fi
[ "${failed}" -eq 0 ]
