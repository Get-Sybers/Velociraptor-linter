## What changed

<!-- one or two sentences -->

## Checklist

- [ ] `make conform` is green locally (yamllint, `lint/Artefacts.yaml`, `velociraptor artifacts verify`, smoke)
- [ ] A change to a lint rule updates `tests/lint/` and `tests/lint/expected.txt` together
- [ ] New or changed artefacts live under `Dev/`; nothing moves to `Prod/` without a collection proving it
- [ ] Long-form documentation lives in a `doc<Name>.yml` companion, not in the artefact
- [ ] Parameter hints say whether the value is a glob or a regex and that it is used verbatim
- [ ] Pins changed only in their single home (`.github/workflows/requirements/`, `.gitmodules`)
