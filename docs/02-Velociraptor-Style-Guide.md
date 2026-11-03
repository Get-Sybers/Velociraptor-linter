# Velociraptor Artefact Style Guide

Velociraptor publishes no style guide for artefacts. Upstream's `docs/style.md` covers Go, and
the documentation site has one prescriptive page, *Artifact Writing Tips*, plus the field
reference. This guide is extrapolated from the bundled definitions in
`upstream/velociraptor/artifacts/definitions` at the pinned commit (423 at the time of writing),
measured with a throwaway script. Where the corpus and the Tips page disagree, the Tips page
wins: it states intent, the corpus accumulates history. [01-VQL-Standards.md](01-VQL-Standards.md)
turns this guide into enforced rules.

## What upstream says

- `name` is the only required field; `description`, `author` and `reference` are strongly
  recommended.
- Names use `a-zA-Z0-9_.`, segments start with a letter, CamelCase by convention.
- The description opens with a lead sentence (never "This artifact..."), carries searchable
  keywords, keeps links in `reference`, and uses `###` headings or lower because the GUI
  reserves `h2` for the name.
- Every parameter has a default and a `description`; `friendly_name` and `validating_regex`
  where they help.
- Name every source when there is more than one; put repeated VQL in `export`.
- Bundled artefacts ship golden tests and run through the verifier in CI.
- Upstream recommends inline VQL comments. This repository does not (see the last section).

## What the corpus does

| Convention | Measurement |
|---|---|
| Field order | `name, description, [type], parameters, sources`; `author` after `name`, `reference` after `description` |
| Name shape | three segments in 369 of 423: `<OS>.<Category>.<Name>` |
| `type` | omitted in about half (defaults to CLIENT); upper case otherwise |
| Parameter names | UpperCamelCase in the large majority |
| Parameter types | untyped string, then `regex`, `bool`, `int`, `timestamp`, `hidden`, `csv`, `yara`, `choices` |
| Glob lists | `csv` with a `Glob` column |
| Precondition | `SELECT OS From info() where OS = 'windows'` at artefact level |
| Sources | named whenever an artefact has several |
| Comments | present in about a quarter of artefacts |
| Output columns | UpperCamelCase in four of five aliases; `_`-prefixed scratch columns (`_Data`, `_value`) and all-caps acronyms (`SHA1`, `URL`) make up most of the rest; `column_types` declared in 56 |
| Logging | `log()` in 59 artefacts; the level is usually omitted, and ERROR calls carry `dedup` |
| Masking | `// linter: symbol_mask_warn:<name>` where a parsed column equals a function name |
| Permissions | `required_permissions` and `implied_permissions` where the verifier reports them |
| Reuse | `export` and `imports`; `Artifact.X()` calls throughout |
| Tests | golden `.in.yaml` / `.out.yaml` pairs under `artifacts/testdata/server/testcases`, run by `velociraptor golden` |

The loader (`services/repository/repository.go`) is the de-facto schema: strict YAML, a valid
name and `type`, parsable VQL, valid `required_permissions`, no alias collisions.
`velociraptor artifacts verify` adds static analysis: unknown plugins and functions, wrong or
missing arguments, permission propagation from called artefacts, symbol masking, invalid
imports. Both read at most 100,000 bytes per artefact.

## The style

1. Identity block first: `name`, `author`, `description`, `type`, `reference`.
2. Dotted UpperCamelCase names, platform first.
3. Explicit upper-case `type`, even though CLIENT is the default: the GUI filters on it.
4. A lead sentence opens the description; headings inside it are `###` or lower.
5. UpperCamelCase parameters, typed where the GUI renders a better control (`regex`, `bool`,
   `timestamp`, `csv`, `multichoice`), each with a default and a hint.
6. Glob lists as `csv` tables with a `Glob` column; single globs as plain strings.
7. An OS precondition on every client artefact; in bundles a precondition per source so the
   selection parameter gates each one.
8. Named sources whenever there is more than one; `_Source` tagging in wrappers.
9. `LET` first, one `SELECT` per source, `<=` to materialise what is reused.
10. Symbol masking resolved by renaming the alias; the `// linter:` directive only when the
    column comes from parsed data.
11. Declared permissions for everything the verifier reports.
12. Verified before upload; proven in a real collection before promotion.
13. UpperCamelCase output columns, upstream's names for upstream's things (`OSPath`, `Mtime`,
    `Hash`), `_` for a column the artefact adds about itself.
14. `log(level="ERROR")` means the collection failed, so it is reserved for that; `dedup` keeps
    a per-row message from flooding the log.

## Where this guide departs

| Topic | Corpus | This repository |
|---|---|---|
| Comments in VQL | common, recommended | forbidden except `// linter:` directives; the prose lives in the doc companion |
| Documentation | long descriptions | a lead paragraph in the artefact, the rest in a `.Doc` NOTEBOOK companion |
| `type` | often omitted | always explicit; the directory and the GUI filter depend on it |
| Parameter suffixes | loose | the suffix fixes the type; the hint names the kind and says the value is used verbatim |
| Parameter order | arbitrary | Selection, Scope, Filter, Output, Server |
| Dependencies | unchecked at load | resolved against the submodule universe, call keywords checked |
| Names | `<OS>.<Category>.<Name>` | `<Namespace>.<Platform>.<Category>.<Name>`; the name is the path |
| Dev/Prod | not applicable | two trees; warnings are errors in `Prod/` |
| Outputs | names vary per artefact | canonical names, `_Source` on every wrapper and bundle source, one column set per bundle |
| Logging | level often omitted | every `log()` names its level; `Error:` only at ERROR |
| Tests | golden tests for bundled artefacts | the linter and verifier, plus a smoke that collects every artefact on impersonated hosts (`tests/smoke/`) |
