# VQL Artefact Standards

These rules govern every artefact in `Dev/` and `Prod/`. `make conform` enforces them with
`yamllint` and the pinned Velociraptor release: a Go linter (`linter/`) implementing this
standard (rule table in the appendix), `velociraptor artifacts verify` and a smoke run. A rule
the gate cannot check yet is a gap in the linter, not an exemption.

One sentence governs the rest: **an artefact collects and a bundle decides; every input arrives
through a typed parameter that the VQL uses verbatim; every artefact is loadable by Velociraptor
itself before it is lintable by us.**

## 1. Names

**Namespace.** Every name begins with a namespace segment identifying the author organisation.
The linter checks the name's *shape*, not a particular prefix, so choose one segment and keep to
it. The `Custom.` prefix the GUI editor adds is never committed.

**Artefacts.** `<Namespace>.<Platform>.<Category>.<Name>[.<Sub>]`: three to six dot-separated
UpperCamelCase segments, `<Platform>` one of `Windows`, `Linux`, `MacOS`, `Generic`, `General`,
`Server`, `Bundle` or `DetectRaptor`. The name is the file path:
`Dev/Client/Example/Windows/Registry/RunKeys.yaml` defines `Example.Windows.Registry.RunKeys`
and nothing else. The directory fixes the `type`: `Client/` holds `CLIENT` and `CLIENT_EVENT`,
`Server/` holds `SERVER` and `SERVER_EVENT`, `Notebooks/` holds `NOTEBOOK`. A name never collides
with a built-in, Exchange or DetectRaptor artefact.

**Doc companions.** `doc<Name>.yml` beside `<Name>.yaml` defines `<Artefact>.Doc`, type
`NOTEBOOK` wherever it lives. Artefacts are `.yaml`; `.yml` is reserved for companions.

**Parameters.** UpperCamelCase (`FileIOCRegex`); an underscore only after a technique or
version number (`T1547_001_RunKeyRegex`). The suffix fixes the type (see Parameters). Our
parameter names are ours, the keyword in a call is theirs:
`Artifact.Windows.Applications.Chrome.History(userRegex=UserRegex)`.

**Variables.** `LET` names follow the parameter rule: UpperCamelCase, or `_` for a value nobody
reads (`LET _ <= ...`). A materialised table ends in `Table`; a function reads as a verb phrase
with UpperCamelCase arguments (`LET ParseLine(Line) = ...`). Lower-case names are the upstream
habit, not ours: a reviewer tells a parameter, a variable and a plugin apart by shape.

**Sources.** Every source of a multi-source artefact is named, uniquely, from `[A-Za-z0-9_.-]`.
A wrapper source is named after the artefact it wraps and every row it emits carries that name
in a `_Source` column, so a bundle's results are attributable without a join.

**Field order.** `name`, `author`, `description`, `type`, `reference`, `required_permissions`,
`resources`, `tools`, `imports`, `export`, `parameters`, `precondition`, `sources`: identity,
prose, contract, code.

## 2. Four shapes

- **Artefact**: one collection concern with its own VQL. It performs; it does not decide.
- **Wrapper**: a thin artefact over a built-in that adds one thing, usually an IOC filter or a
  fixed parameter set, so the filter is written once and every bundle calls the same one.
- **Bundle**: `<Namespace>.Bundle.*` and the multi-source hunters. It decides what runs
  (`SourceSelection`), passes inputs through and tags rows with `_Source`. It carries no
  parsing logic of its own.
- **Doc companion**: the long-form documentation of any of the above (see Documentation).

**Never copy VQL.** A query two artefacts need is one artefact the others call, or an `export`
the others `imports`. A `LET` block copied between files is a defect. A built-in, Exchange or
DetectRaptor artefact is called by its fully-qualified name, never copied under a new name: a
copy forks the logic and stops receiving fixes.

**Static data is data.** IOC lists and lookup tables are `csv` or `json_array` parameters, a
`tools:` entry, or a `LET` table of `dict()` rows, never a nested `if()` ladder.

## 3. Calling other artefacts

Call `Artifact.<Name>()` wherever a bundled, Exchange or DetectRaptor artefact already collects
the thing. The linter resolves every call against this tree and the submodules under
`upstream/`: the bundled definitions, the Artifact Exchange and DetectRaptor. A call outside
that universe is an error, because the collection would fail at launch. Every keyword passed
must be one of the callee's parameters; Velociraptor does not check this at load time, the
linter does.

Exchange and DetectRaptor artefacts must be imported on the server
(`Server.Import.ArtifactExchange`, `Server.Import.DetectRaptor`) before ours that call them are
collected. The doc companion says so.

## 4. Must, must not, should

**Every artefact has**

| Field | Rule |
|---|---|
| `name` | matches the path; not an upstream name |
| `author` | a consistent string for your own work (e.g. `<Org> - <Person>`), the original author for an adapted one |
| `description` | a lead sentence first; at most 12 lines and 1,000 characters |
| `type` | explicit, upper case, matching the directory |
| `reference` | at least one link (warning in `Dev/`, error in `Prod/`); the doc link first when a companion exists |
| `sources` | at least one; all named when there is more than one |
| `precondition` | an OS gate on the artefact or on every client source (warning in `Dev/`) |
| `required_permissions` | every permission the verifier reports (`EXECVE`, ...) |
| `_Source` | projected by every wrapper and bundle source (see Outputs) |

**Never**

| Item | Why |
|---|---|
| comments in YAML or VQL | prose belongs in the companion; the one exception is `// linter: <rule>:<subject>` |
| `reports`, `references`, `queries` | deprecated or aliased upstream; use `reference` and `query` |
| a file over 100,000 bytes | the verifier reads no more |
| a parameter never used in VQL | a dead input misleads the operator (warning) |
| a literal concatenated onto a `*Glob` or `*Regex` parameter | the input is the complete pattern (see Parameters) |
| `LET` inside a `{ ... }` subquery, `LIMIT <variable>` | neither parses; hoist the `LET`, bound with `WHERE count() <= Max` |
| a column alias that masks a VQL function (`url`, `timestamp`, `ip`) | rename it, or declare `// linter: symbol_mask_warn:<name>` |
| secrets, hostnames or site data in a default | use `server_metadata` or `redacted` parameters |
| output columns in snake_case or `prefix_Mixed` | the result is a table analysts join across artefacts (see Outputs) |
| `log()` without a `level`; `Error:` in a message below ERROR | Velociraptor decides pass or fail from them (see Errors and logging) |

**Should**: `resources.timeout` on anything that walks a filesystem; `column_types` for
timestamps, uploads and nested columns; a `notebook` suggestion on hunters whose results need a
summary query.

## 5. Parameters

Parameters are the only inputs. Each has a `name`, a `description` (the GUI hint), a lower-case
`type` where the value is anything but free text, and a `default`, so an artefact run with no
customisation never errors. `timestamp`, `hidden`, `upload` and `server_metadata` parameters
need no default.

**The suffix fixes the type.**

| Name ends in | `type` | Value |
|---|---|---|
| `Regex` | `regex` | one complete regular expression |
| `Glob` | *(omitted)* | one complete glob |
| `Globs` | `csv` (`Glob` column) or `json_array` | several complete globs, one per row |
| `After`, `Before` | `timestamp` | a UTC bound; empty means unbounded |
| `Table`, `List` | `csv` or `json_array` | a static table |
| `Selection` | `multichoice`, `choices` or `artifactset` | which sources or artefacts run |
| `Limit`, `Count`, `Rows`, `Workers`, `Timeout`, `Depth` | `int` | a resource bound |
| `Yara` | `yara` | a YARA rule set |

Conversely a `regex` parameter ends in `Regex` and a `timestamp` in `After`, `Before`, `Time`,
`Timestamp` or `Date`.

**Glob or regex.** A glob selects where the client looks: files, directories, registry keys. It
runs inside the accessor, costs nothing, and belongs to the Scope stage. A regex decides which
rows survive: it runs on values already read and belongs to the Filter stage. So a `*Glob` for
every path the client walks and a `*Regex` for every value it tests, never a regex to find files
nor a glob to test text. A path column is regex-filtered only for what a glob cannot say, such as
a component at an unknown depth or a case-insensitive match on Linux.

**The hint says what the value is.** For `*Glob`, `*Globs` and `*Regex` the hint, in under 500
characters, names the kind, says the value is used verbatim, says whether quotes are needed and
what the operator types themselves (`(?i)`, anchors, `**`, a comma), and gives an example
introduced by `e.g.`, normally the default:

> Regex matched against the service image path; the full expression is used verbatim (add your
> own anchors and flags). No quotes; add (?i) for case-insensitive and anchors as needed, e.g.
> `(?i)^(admin|svc.*)$`.

> Globs, one per row, of the registry keys holding archive history; each full pattern,
> wildcards included, is used verbatim. Quote a row only when it contains a comma; ** recurses,
> e.g. `HKEY_USERS\*\Software\7-Zip\**`.

**Wildcards live in the input.** VQL uses the value as-is (`glob(globs=HistoryGlob)`,
`WHERE Name =~ FileIOCRegex`) and never wraps it in anchors, flags or wildcards; otherwise the
hint lies and two artefacts decorate the same input differently. Joining two complete patterns
with `|` is a union, not a decoration, and is allowed. A command line for `execve()` is the full
`argv` in a `csv` or `json_array` parameter, and the artefact declares `EXECVE`.

**Order: collection first, processing last.** Parameters are listed in the order a collection
consumes them, so the form reads as the data flows and the inputs that cost endpoint time come
first:

1. **Selection**: `SourceSelection`, `artifactset`.
2. **Scope**: `*Glob`, `*Globs`, `*Path`, `*Dir`, `*File`, `*Table`.
3. **Filter**: `*Regex`, `*Yara`, `*After`, `*Before`, `*Time`, `*Date`, `Include*`,
   `Exclude*`, `Only*`.
4. **Output**: `upload` types, `Upload*`, `Max*`, `*Limit`, `*Count`, `*Rows`, `*Workers`,
   `*Timeout`, `*Depth`.
5. **Server**: `hidden`, `server_metadata`, `redacted`.

Unclassified parameters keep their position.

## 6. VQL

- **No comments**, except the verifier directive `// linter: symbol_mask_warn:<name>`, declared
  once at the top of the query and only when the masking column comes from parsed data.
  Otherwise rename the alias (`IP`, `Timestamp`).
- **Preconditions** gate on OS (`SELECT OS FROM info() WHERE OS = "windows"`) and, in a bundle,
  on the selection (`... AND "Amcache" IN SourceSelection`).
- **Structure**: `LET` definitions first, materialised with `<=` when reused, one `SELECT` per
  source. A `LET` never sits inside a `{ ... }` subquery; hoist it as a function
  (`LET ParseLine(Line) = SELECT ...`).
- **Limits**: `LIMIT` takes a literal. A parameterised bound is
  `WHERE ... AND (MaxResults = 0 OR count() <= MaxResults)`; `AND` is lazy, so the count only
  advances on surviving rows.
- **Lookups** are tables (`LET OutcomeTable <= SELECT * FROM foreach(row=[dict(Re=..., Outcome=...), ...])`)
  and a function that selects from them, never a deep `if()`.
- **`_Source` tagging**: every wrapper and bundle source projects `"<name>" AS _Source`.
- **Verbatim inputs** (see Parameters).
- **A plugin is not a function**: `execve()`, `glob()`, `pslist()` yield rows to `FROM`; written
  as a value (`LET Out <= execve(...)`) the symbol is not found at run time and the verifier does
  not notice (VS080).
- **Only plugins the pinned release has**: `FROM amcache()` parses and verifies and fails on the
  endpoint; the linter checks every `FROM name()` against the release's VQL reference (VS081).
- **Sources are not variables**: `FROM OtherSource` inside a sibling source is a symbol that
  does not exist at collection time; union sources in a notebook with `source(source=...)`, or
  call the artefact with `source=` (VS082).
- **A call runs every source**: `Artifact.X()` ignores the preconditions of X's sources, so a
  multi-platform X called from a Linux host runs its Windows sources too and logs their
  errors; pass `preconditions=TRUE`, or `source=` for the one source wanted (VS083).
- **A YARA regex has delimiters**: a value placed between the `/` of a rule's regex string
  must have `/` escaped first (`regex_replace(source=X, re="/", replace="\\/")`); an unescaped
  `/` ends the pattern early and the whole rule fails to compile (VS084).

**Flow of execution.** Everything that shrinks the result runs on the client, cheapest first, so
endpoint time and bandwidth go to the rows that matter:

1. **Scope**: `glob()` over the `*Glob` and `*Globs` parameters; the accessor does the walking.
2. **Cheap filters**: name, size, the `*After` and `*Before` bounds, the `*Regex` parameters on
   columns already in hand.
3. **Expensive filters**: `yara()`, `parse_*()`, `read_file()`, only on rows that passed the
   cheap ones; `AND` is lazy, so order the clauses that way.
4. **Deduplication**: `GROUP BY` the identity of the thing (path, hash, key) before anything is
   hashed or uploaded twice.
5. **Client-side enrichment**: `hash()`, `authenticode()`, `parse_pe()`, on surviving rows only.
6. **Upload** last, bounded by `Max*` and `*Limit`.

Enrichment that needs what only the server knows, such as a hunt-wide join, threat intelligence,
`server_metadata` or geo-location, lives in a notebook template or a SERVER artefact, never in
the client query. The client artefact ships the key the server will join on: hash, path, SID.

## 7. Outputs

**Names.** Output columns are UpperCamelCase (`OSPath`, `CommandLine`); an all-caps acronym
stays all-caps (`SHA256`, `UPN`, `URL`); a column the artefact adds about itself starts with
`_` (`_Source`, `_Technique`, `_Hunter`). Never snake_case or `prefix_Mixed`: `git_UserName`
is `GitUserName`. Use upstream's name for upstream's thing, so results from different artefacts
join without renaming:

| Thing | Column |
|---|---|
| a path on the endpoint | `OSPath` (never `FullPath`, `Path` or `File`) |
| file times | `Mtime`, `Atime`, `Ctime`, `Btime` |
| size in bytes | `Size` |
| a digest | `Hash` (the dict `hash()` returns) or `MD5`, `SHA1`, `SHA256` |
| when the event happened | `Timestamp`; `EventTime` for log records |
| who | `Username`, `UserSID` |
| the host | `Hostname`, `Fqdn` (hunt results get `ClientId` and `Fqdn` from Velociraptor itself) |
| a process | `Pid`, `Ppid`, `Name`, `Exe`, `CommandLine` |
| the file shipped back | `Upload` |
| where the row came from | `_Source` |

**Order.** Metadata first, payload last, identity before detail: the `_*` columns, then what the
row is (`OSPath`, `Name`, `Key`, `Pid`), then when (`Timestamp`, `Mtime`), then the details, then
`Upload`, `Content`, `Parsed` and other payloads. The GUI shows columns in projection order, so
an analyst reads provenance, identity, time, detail.

**Mandatory.** Every wrapper and bundle source projects `_Source`: the wrapped artefact's name in
a wrapper, the source name in a bundle. A pass-through `SELECT * FROM Artifact.X()` carries the
`_Source` Velociraptor adds to every row an artefact call returns.

**Bundles.** Every source of a `<Namespace>.Bundle.*` artefact projects the same columns in the
same order, with `NULL` where a source has no value, so the sources union into one table and the
GUI, which sizes the table on the first rows, never drops a column. A value only one source has
goes into a `Details` dict column, not a column of its own.

**Nested data.** A dict stays a dict: the GUI expands it, JSON export keeps it, and the
`column_types` `json/N`, `tree` and `collapsed` say how it is shown. Promote a key an operator
filters or pivots on to a top-level column. Never `serialize()` a dict into a string for display
and never flatten one into `Prefix_Key` columns. CSV export writes nested values as JSON text, so
a bundle meant for CSV keeps its rows flat.

## 8. Errors and logging

Velociraptor marks a collection failed when its VQL logs at ERROR, or logs any message matching
its error regex (`Error:`, `Symbol ... not found`, `Field ... is required`, `Unexpected arg`),
and the query keeps running. The flag is the artefact's verdict on its own collection, so:

- `log(level="ERROR", message="...", args=...)` only for a condition that invalidates the
  collection: a required tool or file missing, a parse that drops data, a permission the
  artefact needs and lacks. The operator sees the flow turn red with the message.
- `level="WARN"` for a degraded result the operator should know about, such as a source skipped
  or a bound reached; `level="INFO"` for progress; `level="DEBUG"` for detail. A message below
  ERROR never contains `Error:`.
- The message is a format string with `args`, never concatenation. `dedup=-1` when the message
  carries per-row data that must not be suppressed; otherwise the default suppresses repeats of
  the same format string for 60 seconds.
- A condition that merely does not apply is a precondition, not a log. Zero rows is not an
  error.

`verify()` from VQL and `Server.Utils.ArtifactVerifier` on a live server run the same static
checks as the gate's `make verify`.

## 9. Documentation

**The artefact carries the lead.** `description` opens with one sentence that says what the
artefact reports, keywords included, never "This artefact". It stays within 12 lines and 1,000
characters so the collection wizard shows the point.

**The companion carries the rest.** `doc<Name>.yml` is a `NOTEBOOK` artefact named
`<Artefact>.Doc` whose `description` is the full documentation in CommonMark: sources, inputs,
caveats, timestamp formats, server prerequisites, and an "Implementation notes" section where
VQL comments would have been. Its single markdown cell renders that description, so a notebook
created from it opens on the documentation (verified on 0.77.3):

```yaml
sources:
  - notebook:
      - type: markdown
        name: Documentation
        template: |
          {{ range Query "SELECT description FROM artifact_definitions(names='Example.X.Doc')" | Expand }}{{ Get . "description" }}{{ end }}
```

The artefact links its companion first in `reference` as
`"[Documentation](#/artifacts/Example.X.Doc)"`, which the GUI renders as an in-app link.
Because the companion has a source it appears in *View Artifacts* and the notebook-template
picker; because it is `NOTEBOOK` it never appears in a client collection list.

## 10. Dev and Prod

`Dev/` holds everything waiting to be proven, `Prod/` everything proven in a collection.

| | `Dev/` | `Prod/` |
|---|---|---|
| errors | fail the gate | fail the gate |
| warnings (VS006, VS009, VS022, VS034, VS037, VS038, VS039, VS050 to VS053, VS060 to VS062) | reported | fail the gate |
| smoke failure (an error logged on a smoke host) | reported | fail the gate |
| dependencies | this tree, built-ins, Exchange, DetectRaptor | `Prod/`, built-ins, Exchange, DetectRaptor; never `Dev/` |

Promotion is a pull request that moves the artefact and its companion to the same path under
`Prod/` and states the collection (client, flow id, OS version) that proved it; the smoke must
be clean for it. The same name may exist in both trees while a new version is in `Dev/`; the
two trees are loaded into different servers.

## 11. The gate

`make conform` runs the checks below, cheapest first. CI runs the same targets and `all_green` is
the one required status check; the [README](../README.md) covers the targets, CI and pins.

- **`make yamllint`**: `.yamllint`, strict.
- **`make lint`**: the Go linter (`linter/`), this document's rules (appendix), run over the
  YAML on disk. It reuses the pinned release's VQL parser for the AST rules and resolves calls
  and collisions against a dumped index of the built-ins, Exchange and DetectRaptor. Warnings
  are errors under `--strict` and always under `Prod/`. `make lint-test` runs it over the
  fixtures under `tests/lint/`, one broken artefact per rule group, which must produce exactly
  `tests/lint/expected.txt`; a rule change updates the rule, its fixture and expected.txt.
- **`make verify`**: `velociraptor artifacts verify` from the pinned release, with the Exchange
  and DetectRaptor definitions loaded: the same code path as a GUI upload. Strict YAML, VQL
  parsing of every `query`, `precondition` and `export`, unknown plugins and functions, wrong or
  missing arguments, missing `required_permissions`, symbol masking, invalid imports; warnings
  are errors. It does not check that a called artefact exists, that call keywords are its
  parameters, or that a name collides with a built-in. The linter does.
- **`make smoke`**: every client artefact, found with `artifact_definitions()` and collected
  with `collect()` (`tests/smoke/harness.vql`), once on each of three hosts the pinned release
  impersonates through its dead-disk remapping (`--remap`, `tests/smoke/<host>.yaml`): Windows,
  Linux and macOS, each a directory holding the sample files Velociraptor's own test suite
  ships (`tests/smoke/files.txt`), the sample hives mounted into the registry, and the plugins
  that read the live machine disabled. The verdict is Velociraptor's: a log line at ERROR, or
  one matching the patterns its flow runner and golden runner call fatal. The test knows no
  artefact by name; adding one adds nothing to `tests/`. A failure is reported for an
  artefact in `Dev/` and fails the gate for one in `Prod/` (or any under `--strict`), like a
  warning. An artefact whose `tools:` entry has no `url` is skipped: no inventory serves it
  here. Server and event artefacts are not run.

Suppression is the verifier's own directive, per subject, in the artefact. There is no
repo-wide skip list; a wrong rule is changed in the linter (`linter/rules/`) through a reviewed
pull request.

To try an artefact against this machine before writing its test:

```sh
scripts/velociraptor.sh --definitions Dev/Client artifacts collect <Name> --format json
```

## 12. Related documents

- [02-Velociraptor-Style-Guide.md](02-Velociraptor-Style-Guide.md) — the style this standard
  turns into enforced rules, measured against Velociraptor's own bundled corpus.
- [03-Linting-and-CI.md](03-Linting-and-CI.md) — the gate, the pins and the CI that run it.

## Appendix: linter rules

| Rule | Severity | Checks |
|---|---|---|
| VS001 | error | YAML parses; top level is a mapping |
| VS002 | error | path is `Dev\|Prod/Client\|Server\|Notebooks/…`; artefacts `.yaml`, companions `doc*.yml` |
| VS003 | error | `name` matches the path |
| VS004 | error | name charset, UpperCamel segments, platform segment, 3–6 segments |
| VS005 | error | `type` explicit, upper case, valid, matches directory; companions are `NOTEBOOK` |
| VS006 | error / warning | `description`, `author`, `sources` present; `reference` present (warning) |
| VS007 | error | no `reports`, `references`, `queries` |
| VS008 | error | no YAML comments |
| VS009 | error / warning | ≤ 100,000 bytes (warning from 80,000) |
| VS010 | error | lead sentence; ≤ 12 lines / 1,000 chars; no "This artefact" |
| VS011 | error | `reference` links an existing companion |
| VS012 | error | companion has its artefact, one `notebook` source, a markdown first cell, no parameters |
| VS020 | error | multi-source artefacts name every source; unique; valid charset |
| VS021 | error | every source has a `query` (NOTEBOOK excepted) |
| VS022 | warning | client sources have an OS precondition |
| VS023 | error | no VQL comments except `// linter:` directives |
| VS024 | error | `Artifact.X()` resolves in the tree or the universe |
| VS025 | error | keywords passed to `Artifact.X()` are its parameters |
| VS026 | error | name does not collide with a built-in, Exchange or DetectRaptor artefact |
| VS030 | error | parameter names UpperCamel, unique |
| VS031 | error | every parameter has a hint |
| VS032 | error | every parameter has a default (timestamp, hidden, upload, server_metadata exempt) |
| VS033 | error | suffix ↔ type agreement; lower-case types |
| VS034 | error / warning | pattern hints name the kind; say the value is used verbatim (warning) |
| VS035 | error | no literal concatenated onto a pattern parameter (`\|` unions allowed) |
| VS036 | error | stage order Selection > Scope > Filter > Output > Server |
| VS037 | warning | hint ≤ 500 characters |
| VS038 | warning | parameter is used in VQL (a notebook's `vql` cells count) |
| VS039 | warning | pattern hints say whether quotes are needed and give an `e.g.` example |
| VS040 | error | a `Prod/` artefact never depends on a `Dev/`-only artefact |
| VS042 | error | a name is defined once per tree |
| VS050 | warning | output columns are UpperCamelCase, an all-caps acronym, or `_`-prefixed metadata |
| VS051 | warning | every wrapper, bundle and multi-source source projects `_Source` |
| VS052 | warning | every source of a `Bundle` artefact projects the same columns in the same order |
| VS053 | warning | `_*` columns first, payload columns (`Upload`, `Content`, `Parsed`, `Raw*`) last |
| VS060 | warning | `log()` passes a `level` Velociraptor knows |
| VS061 | warning | a message below ERROR carries no error text (`Error:` ...) |
| VS062 | warning | `LET` names are UpperCamelCase (`_` for scratch values) |
| VS080 | error | a plugin is not assigned or used as a value (`LET X <= execve(...)`) |
| VS081 | error | every `FROM name()` is a plugin of the pinned release or a `LET` of the artefact |
| VS082 | error | no source reads a sibling source as a variable (`FROM OtherSource`) |
| VS083 | warning | a call to an artefact whose sources carry different preconditions passes `preconditions=TRUE` or `source=` |
| VS084 | error | a value placed between the `/` delimiters of a YARA regex (`/%v/`, `$s = /" + X`) has `/` escaped in the same artefact |

Warnings are errors under `--strict` and always for files under `Prod/`.
