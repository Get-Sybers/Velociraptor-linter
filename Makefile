# Velociraptor artefact conformance. `make conform` is the gate CI runs and the
# one command a contributor runs locally; every target below is a piece of it.
# The linter is a Go program under linter/; yamllint, the verifier and the smoke
# run through the pinned Velociraptor release.
PY ?= python3
VR := scripts/velociraptor.sh
ARTEFACTS := Dev Prod
REQ := .github/workflows/requirements/python.txt
DEFS := .cache/definitions
LINT := .cache/velociraptor-lint
UNIVERSE := .cache/universe.jsonl
REFERENCE := upstream/velociraptor/docs/references/vql.yaml

.PHONY: conform deps tools submodules yamllint lint lint-bin lint-test lint-test-update verify smoke upgrade clean

conform: yamllint lint lint-test verify smoke

deps:
	$(PY) -m pip install --quiet --ignore-installed --requirement $(REQ)

tools:
	@$(VR) > /dev/null

submodules:
	git submodule sync --quiet --recursive
	git submodule update --init --recursive --depth 1

yamllint:
	$(PY) -m yamllint --strict .

# Exchange and DetectRaptor definitions: loaded by the verifier and the smoke so
# called-artefact permissions propagate, and dumped into the linter's universe.
$(DEFS): $(wildcard upstream/velociraptor-docs/content/exchange/artifacts/*.yaml) $(wildcard upstream/DetectRaptor/vql/*.yaml)
	rm -rf $(DEFS) && mkdir -p $(DEFS)/exchange $(DEFS)/detectraptor
	cp upstream/velociraptor-docs/content/exchange/artifacts/*.yaml $(DEFS)/exchange/
	cp upstream/DetectRaptor/vql/*.yaml $(DEFS)/detectraptor/

# The Go linter binary.
$(LINT): linter/go.mod linter/go.sum $(wildcard linter/cmd/velociraptor-lint/*.go) $(wildcard linter/model/*.go) $(wildcard linter/rules/*.go)
	@mkdir -p .cache
	cd linter && go build -o "$(CURDIR)/$(LINT)" ./cmd/velociraptor-lint

lint-bin: $(LINT)

# The definitions index the resolution rules (VS024/025/026/040/083) resolve
# against, dumped from the pinned release with Exchange and DetectRaptor loaded.
$(UNIVERSE): tools $(DEFS) scripts/universe.vql
	$(VR) --definitions $(DEFS) query --format jsonl "$$(cat scripts/universe.vql)" > $(UNIVERSE)

# The standard as a Go linter, run over the artefact trees (Dev, Prod).
lint: $(LINT) $(UNIVERSE)
	$(LINT) --universe $(UNIVERSE) --reference $(REFERENCE) $(ARTEFACTS)

# The linter's own regression test: the fixtures under tests/lint produce
# exactly tests/lint/expected.txt.
lint-test: $(LINT) $(UNIVERSE)
	$(LINT) --root tests/lint --universe $(UNIVERSE) --reference $(REFERENCE) --expect tests/lint/expected.txt Dev Prod

lint-test-update: $(LINT) $(UNIVERSE)
	$(LINT) --root tests/lint --universe $(UNIVERSE) --reference $(REFERENCE) --update-expected tests/lint/expected.txt Dev Prod

verify: tools $(DEFS)
	$(VR) --definitions $(DEFS) artifacts verify --nocolor $(ARTEFACTS)

# Smoke: every client artefact collected once on every smoke host through the
# pinned release; an error in the log fails it. With no artefacts in the trees
# it is a no-op, and it covers whatever is added without naming any.
smoke: tools $(DEFS)
	scripts/smoke.sh

upgrade:
	scripts/upgrade.sh

clean:
	rm -rf .cache
