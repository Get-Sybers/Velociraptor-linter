# Velociraptor artefact conformance. `make conform` is the gate CI runs and the
# one command a contributor runs locally; every target below is a piece of it.
# Everything but yamllint runs through the pinned Velociraptor release.
PY ?= python3
VR := scripts/velociraptor.sh
ARTEFACTS := Dev Prod
REQ := .github/workflows/requirements/python.txt

.PHONY: conform deps tools submodules yamllint vql-lint lint-test lint-test-update verify smoke upgrade clean

conform: yamllint vql-lint lint-test verify smoke

deps:
	$(PY) -m pip install --quiet --ignore-installed --requirement $(REQ)

tools:
	@$(VR) > /dev/null

submodules:
	git submodule sync --quiet --recursive
	git submodule update --init --recursive --depth 1

yamllint:
	$(PY) -m yamllint --strict .

# Exchange, DetectRaptor and the lint artefact are loaded alongside the
# built-ins: the first two so permissions declared by called artefacts
# propagate into the analysis and calls resolve, the last so the linter runs.
DEFS := .cache/definitions
$(DEFS): $(wildcard upstream/velociraptor-docs/content/exchange/artifacts/*.yaml) $(wildcard upstream/DetectRaptor/vql/*.yaml) $(wildcard lint/*.yaml)
	rm -rf $(DEFS) && mkdir -p $(DEFS)/exchange $(DEFS)/detectraptor $(DEFS)/lint
	cp upstream/velociraptor-docs/content/exchange/artifacts/*.yaml $(DEFS)/exchange/
	cp upstream/DetectRaptor/vql/*.yaml $(DEFS)/detectraptor/
	cp lint/*.yaml $(DEFS)/lint/

# The standard as VQL (lint/Artefacts.yaml), run by the pinned binary over the
# artefact trees (Dev, Prod).
vql-lint: tools $(DEFS)
	scripts/lint.sh $(ARTEFACTS)

# The linter's own regression test: the fixtures under tests/lint produce
# exactly tests/lint/expected.txt.
lint-test: tools $(DEFS)
	scripts/lint.sh --root tests/lint --expect tests/lint/expected.txt Dev Prod

lint-test-update: tools $(DEFS)
	scripts/lint.sh --root tests/lint --update-expected tests/lint/expected.txt Dev Prod

verify: tools $(DEFS)
	$(VR) --definitions $(DEFS) artifacts verify --nocolor $(ARTEFACTS)
	$(VR) --definitions $(DEFS) artifacts verify --builtin --nocolor lint

# Smoke: every client artefact collected once on every smoke host (Windows,
# Linux, macOS impersonated over Velociraptor's sample files) through the
# pinned release; an error in the log fails it. With no artefacts in the
# trees it is a no-op, and it covers whatever is added without naming any.
smoke: tools $(DEFS)
	scripts/smoke.sh

upgrade:
	scripts/upgrade.sh

clean:
	rm -rf .cache
