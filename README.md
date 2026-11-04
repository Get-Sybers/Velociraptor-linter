# Velociraptor Linter

A style guide and conformance gate for [Velociraptor](https://docs.velociraptor.app)
artefacts. The linter is a Go program (`linter/`) that reuses Velociraptor's own VQL parser,
the verifier is Velociraptor's own, and the gate — `make conform` — runs the linter plus the
pinned Velociraptor release, so artefacts are checked with the exact engine that loads them.

Point it at a tree of artefacts and it enforces consistent names, layout, parameters,
outputs, documentation and VQL, and flags VQL that parses and verifies but still cannot run
on an endpoint.

## Layout

```text
Dev/                 artefacts waiting to be proven
Prod/                artefacts proven in a collection; warnings are errors, never depends on Dev/
  Client/            CLIENT and CLIENT_EVENT
  Server/            SERVER and SERVER_EVENT
  Notebooks/         NOTEBOOK templates
linter/              the linter: a Go program implementing the standard (rule table is the docs/01 appendix)
  cmd/velociraptor-lint/   the CLI
  model/, rules/           artefact model + one file per rule group
scripts/
  universe.vql       dumps the definitions index the resolution rules resolve against
  smoke.sh           collects every artefact on three impersonated hosts (velociraptor --remap)
  definitions.sh     assembles the definitions directory the verifier and smoke load
  velociraptor.sh    downloads the pinned Velociraptor release into .cache/ and runs it
  upgrade.sh         raises the submodule and Velociraptor pins to their ceiling, then runs the gate
tests/lint/          linter fixtures, one broken artefact per rule group, and expected.txt they must produce
tests/smoke/         the smoke: harness.vql, one remapping per host, files.txt (the sample files each host holds)
upstream/            shallow git submodules, the universe the linter resolves Artifact.X() calls against
  velociraptor/            VQL reference, verifier source and the smoke sample files
  velociraptor-docs/       the Artifact Exchange
  DetectRaptor/            DetectRaptor artefacts
docs/                the standards (see below)
.github/workflows/   ci.yml (the gate), update-pins.yml (weekly pin raise), requirements/ (the pin files)
Makefile             the targets below
```

An artefact's name is its path, and the directory fixes its `type`. Long-form documentation
lives in a `doc<Name>.yml` companion beside it, a `NOTEBOOK` artefact named `<Artefact>.Doc`:

```text
Dev/Client/Example/Windows/Registry/RunKeys.yaml      Example.Windows.Registry.RunKeys      CLIENT
Dev/Client/Example/Windows/Registry/docRunKeys.yml    Example.Windows.Registry.RunKeys.Doc  NOTEBOOK
```

`Dev/` and `Prod/` ship empty: add artefacts and the gate covers them with no further wiring.

## Quick start

```sh
git clone --recurse-submodules --shallow-submodules <repo-url>
cd Velociraptor-linter
make deps      # PyYAML and yamllint, pinned in .github/workflows/requirements/python.txt
make conform   # yamllint, the Go linter, the verifier, the smoke (needs a Go toolchain)
```

`make conform` is the gate CI runs on every pull request and the one command to run locally.
On first use it downloads the pinned Velociraptor release into `.cache/`.

| Target | What it does |
|---|---|
| `make conform` | `yamllint`, `lint`, `lint-test`, `verify`, `smoke`, in that order |
| `make yamllint` | YAML style, strict, from `.yamllint` |
| `make lint` | the Go linter (`linter/`): naming, layout, must-haves, parameters, comment-free VQL, outputs, logging, dependency resolution, Dev/Prod rules |
| `make lint-test` | the linter over `tests/lint/`, which must produce exactly `tests/lint/expected.txt` |
| `make lint-test-update` | rewrite `tests/lint/expected.txt` after a deliberate rule change |
| `make verify` | `velociraptor artifacts verify` from the pinned release, with the Exchange and DetectRaptor definitions loaded |
| `make smoke` | every client artefact collected once on each impersonated host; a logged error fails it (reported in `Dev/`, fatal in `Prod/`) |
| `make upgrade` | raise the submodule and Velociraptor pins and rerun the gate |
| `make submodules` | initialise or refresh the submodules, recursive and shallow |
| `make clean` | remove `.cache/` |

## Documentation

| Document | What it is |
|---|---|
| [docs/01-VQL-Standards.md](docs/01-VQL-Standards.md) | the standard every artefact follows, with the linter's rule table |
| [docs/02-Velociraptor-Style-Guide.md](docs/02-Velociraptor-Style-Guide.md) | the style extrapolated from Velociraptor's bundled artefacts and upstream's writing tips |
| [docs/03-Linting-and-CI.md](docs/03-Linting-and-CI.md) | the gate, the pins and the CI that run it |

## License

No license is set yet. Add one (for example Apache-2.0) before publishing.
