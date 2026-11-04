package rules

import "velociraptor-linter/model"

// VS001 — YAML parses and the top level is a mapping.
func init() { register(vs001) }
func vs001(a *model.Artefact, cx *Context) []model.Finding {
	if a.ParsedOK {
		return nil
	}
	return []model.Finding{line1(a, "VS001", "error",
		"YAML does not parse, or the top level is not a mapping")}
}

// VS002 — artefacts live under Dev|Prod/Client|Server|Notebooks/; artefacts are
// .yaml, companions doc*.yml.
func init() { register(vs002) }
func vs002(a *model.Artefact, cx *Context) []model.Finding {
	layout := a.LayoutOK()
	badExt := !a.IsDoc && !a.IsYAMLExt()
	if layout && !badExt {
		return nil
	}
	var msg string
	switch {
	case !layout:
		msg = "artefacts live under Dev|Prod/Client|Server|Notebooks/"
	case a.IsYMLExt():
		msg = "artefact files use .yaml; .yml is reserved for doc<Name>.yml companions"
	default:
		msg = "unexpected file extension"
	}
	return []model.Finding{line1(a, "VS002", "error", msg)}
}
