package rules

import (
	"fmt"
	"regexp"

	"velociraptor-linter/model"
)

var reSrcName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// VS020/VS021/VS022, the source-level half of VS007, and duplicate source names.
func init() { register(vsSources) }
func vsSources(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc || !a.SourcesIsList {
		return nil
	}
	tu := typeU(a)
	isNotebook := tu == "NOTEBOOK"
	isClient := reClientType.MatchString(tu)
	multi := len(a.Sources) > 1
	var out []model.Finding
	nameCount := map[string]int{}
	for i, s := range a.Sources {
		pfx := fmt.Sprintf("source %d: ", i+1)
		if !s.IsDict {
			out = append(out, line1(a, "VS021", "error", pfx+"each source is a mapping"))
		} else {
			if s.Has("queries") {
				out = append(out, line1(a, "VS007", "error", pfx+"`queries` is not allowed in a source: use `query`"))
			}
			if !isNotebook && !(s.QueryIsStr && reNonSpace.MatchString(s.Query)) {
				out = append(out, line1(a, "VS021", "error", pfx+"source has no `query`"))
			}
			if !isNotebook && multi && !(s.NameIsStr && s.Name != "") {
				out = append(out, line1(a, "VS020", "error", pfx+"every source of a multi-source artefact is named"))
			}
			if !isNotebook && multi && s.NameIsStr && s.Name != "" && !reSrcName.MatchString(s.Name) {
				out = append(out, line1(a, "VS020", "error", pfx+fmt.Sprintf("source name `%v` has invalid characters", s.Name)))
			}
			if isClient && !a.PrecondIsStr && !s.PrecondIsStr {
				out = append(out, line1(a, "VS022", sev(a.Env, cx.Strict), pfx+"client source has no precondition (OS gate) and the artefact has none"))
			}
		}
		if s.NameIsStr && s.Name != "" {
			nameCount[s.Name]++
		}
	}
	for name, c := range nameCount {
		if c > 1 {
			out = append(out, line1(a, "VS020", "error", fmt.Sprintf("duplicate source name `%v`", name)))
		}
	}
	return out
}
