package rules

import (
	"fmt"
	"regexp"

	"velociraptor-linter/model"
)

var rePipeLit = regexp.MustCompile(`^.\|.$`)

func concatRes(name string) []*regexp.Regexp {
	q := regexp.QuoteMeta(name)
	return []*regexp.Regexp{
		regexp.MustCompile(`("[^"]*"|'[^']*')\s*\+\s*` + q + `\b`),
		regexp.MustCompile(`\b` + q + `\s*\+\s*("[^"]*"|'[^']*')`),
	}
}

// VS035 — a literal concatenated onto a *Glob/*Globs/*Regex parameter in VQL.
func init() { register(vs035) }
func vs035(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc || !a.ParamsIsList {
		return nil
	}
	raw := allVQL(a)
	var out []model.Finding
	for _, p := range a.Parameters {
		if !(p.IsDict && p.NameIsStr) || !rePatternSuf.MatchString(p.Name) {
			continue
		}
		hit := false
		for _, re := range concatRes(p.Name) {
			for _, m := range re.FindAllStringSubmatch(raw, -1) {
				if !rePipeLit.MatchString(m[1]) {
					hit = true
					break
				}
			}
			if hit {
				break
			}
		}
		if hit {
			out = append(out, line1(a, "VS035", "error",
				fmt.Sprintf("`%v` is concatenated with a literal in VQL; the user input holds the complete pattern and is used verbatim", p.Name)))
		}
	}
	return out
}

// VS038 — a parameter never referenced in the VQL (strings/comments masked).
func init() { register(vs038) }
func vs038(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc || !a.ParamsIsList {
		return nil
	}
	masked := mask(allVQLAndCells(a))
	s := sev(a.Env, cx.Strict)
	var out []model.Finding
	for _, p := range a.Parameters {
		if !(p.IsDict && p.NameIsStr) {
			continue
		}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(p.Name) + `\b`)
		if !re.MatchString(masked) {
			out = append(out, line1(a, "VS038", s,
				fmt.Sprintf("parameter `%v` is never used in VQL", p.Name)))
		}
	}
	return out
}
