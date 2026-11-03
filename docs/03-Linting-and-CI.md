# Linting and CI

How this repository checks itself and keeps its pins current. The rules the linter
enforces are in [01-VQL-Standards.md](01-VQL-Standards.md) (§11 covers each check); the
make targets are listed in the [README](../README.md). This document covers the CI, the
pins and the dependency universe behind them.

## One pinned release

Everything but `yamllint` runs through one pinned Velociraptor release, so a contributor
and CI check artefacts with the exact binary that will load them. `scripts/velociraptor.sh`
downloads that release into `.cache/` (git-ignored) on first use; CI caches the same
directory, keyed on the pin file.

## Continuous integration

- `.github/workflows/ci.yml` runs `make yamllint` in one job and `make vql-lint`,
  `make lint-test`, `make verify` and `make smoke` in another, on every pull request and
  push to `main`. `all_green` is the one status check branch protection needs.
- `.pre-commit-config.yaml` runs yamllint, gitleaks, the linter and the verifier on staged
  artefacts (`prek install` or `pre-commit install`). Its rev pins are the single source of
  linter versions.
- `.claude/hooks/session-start.sh` gives Claude Code cloud sessions the submodules, Python
  tooling and pinned binary, so `make conform` works from the first turn.

Branches are `<type>/<description>`, pull requests squash-merge, and the PR title is a
Conventional Commit (`feat:`, `fix:`, `docs:`, `chore:`).

## The artefact trees

The linter and smoke run over two trees, `Dev/` and `Prod/`, each split into `Client/`,
`Server/` and `Notebooks/`. They ship empty: drop artefacts in and the gate covers them
with no further wiring — the smoke derives its artefact set from the trees, so it needs no
per-artefact configuration. `Dev/` holds everything waiting to be proven; `Prod/` everything
proven in a collection, where linter warnings become errors and nothing may depend on a
`Dev/`-only artefact. See [01-VQL-Standards.md](01-VQL-Standards.md) §10.

## Pins, each with one home

| Pin | Home | Raised by |
|---|---|---|
| upstream submodules | `.gitmodules` | `update-pins.yml` (weekly), `make upgrade` |
| Velociraptor release | `.github/workflows/requirements/velociraptor.txt` | `update-pins.yml`, `make upgrade` |
| Python tooling | `.github/workflows/requirements/python.txt` | Dependabot (pip) |
| GitHub Actions | the workflow files (SHA-pinned, `# vX.Y.Z` comment) | Dependabot (github-actions) |
| pre-commit hooks | `.pre-commit-config.yaml` | `prek` / `pre-commit` autoupdate |

`update-pins.yml` runs `make upgrade` weekly — raising the submodules to the tip of their
branch and the Velociraptor binary to its newest release — and pushes a `chore-update-pins`
branch for a manual pull request.

## The dependency universe

The linter resolves `Artifact.X()` calls and checks name collisions against the upstream
definitions kept as shallow submodules under `upstream/`:

- `velociraptor` — the pinned source: the VQL reference the improper-method rules check
  against, and the sample files the smoke hosts are built from.
- `velociraptor-docs` — the Artifact Exchange.
- `DetectRaptor` — the DetectRaptor artefacts.

A call to anything outside that universe and the repo's own trees is a lint error, because
the collection would fail at launch.
