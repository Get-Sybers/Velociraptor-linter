package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"velociraptor-linter/model"
)

var (
	reType       = regexp.MustCompile(`^(CLIENT|CLIENT_EVENT|SERVER|SERVER_EVENT|NOTEBOOK|INTERNAL)$`)
	reClientType = regexp.MustCompile(`^CLIENT(_EVENT)?$`)
	reServerType = regexp.MustCompile(`^SERVER(_EVENT)?$`)
	reNonSpace   = regexp.MustCompile(`\S`)
	reCommentLn  = regexp.MustCompile(`^\s*#`)
	reThisArt    = regexp.MustCompile(`(?i)^\s*this (artefact|artifact)`)
	reDescTrim   = regexp.MustCompile(`^\n+|\n+$`)
)

// typeU returns the artefact type upper-cased, defaulting to CLIENT.
func typeU(a *model.Artefact) string {
	if a.Type == "" {
		return "CLIENT"
	}
	return strings.ToUpper(a.Type)
}

// VS005 — type explicit, upper-case, valid, matches directory.
func init() { register(vs005) }
func vs005(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	if !a.HasKey("type") || a.Type == "" {
		return []model.Finding{line1(a, "VS005", "error", "`type` must be set explicitly")}
	}
	tu := strings.ToUpper(a.Type)
	var out []model.Finding
	if a.Type != tu {
		out = append(out, line1(a, "VS005", "error", "`type` is written in upper case"))
	}
	switch {
	case !reType.MatchString(tu):
		out = append(out, line1(a, "VS005", "error", fmt.Sprintf("unknown type `%v`", a.Type)))
	case a.IsDoc:
		if tu != "NOTEBOOK" {
			out = append(out, line1(a, "VS005", "error", "doc companions are `type: NOTEBOOK`"))
		}
	case (a.Kind == "Client" && !reClientType.MatchString(tu)) ||
		(a.Kind == "Server" && !reServerType.MatchString(tu)) ||
		(a.Kind == "Notebooks" && tu != "NOTEBOOK"):
		out = append(out, line1(a, "VS005", "error", fmt.Sprintf("type `%v` does not belong under %v/", a.Type, a.Kind)))
	}
	return out
}

// VS006 (and the top-level half of VS007) — required fields, forbidden keys.
func init() { register(vs006) }
func vs006(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	var out []model.Finding
	if a.HasKey("reports") {
		out = append(out, line1(a, "VS007", "error", "`reports` is not allowed: reports are deprecated upstream"))
	}
	if a.HasKey("references") {
		out = append(out, line1(a, "VS007", "error", "`references` is not allowed: use `reference`"))
	}
	if !(a.DescIsStr && reNonSpace.MatchString(a.Description)) {
		out = append(out, line1(a, "VS006", "error", "`description` is required and must be a non-empty string"))
	}
	if !(a.AuthorIsStr && reNonSpace.MatchString(a.Author)) {
		out = append(out, line1(a, "VS006", "error", "`author` is required and must be a non-empty string"))
	}
	if !a.HasKey("reference") && !a.IsDoc {
		out = append(out, line1(a, "VS006", sev(a.Env, cx.Strict), "`reference` is missing (should have at least one link)"))
	}
	if a.HasKey("reference") && !a.RefIsList() {
		out = append(out, line1(a, "VS006", "error", "`reference` is a list"))
	}
	if !a.SourcesIsList || len(a.Sources) == 0 {
		out = append(out, line1(a, "VS006", "error", "`sources` must be a non-empty list"))
	}
	return out
}

// VS008 — no YAML comments; reports the real line. The serialized form guards
// against flagging a `#` line that is actually data (e.g. a markdown heading in
// a doc template).
func init() { register(vs008) }
func vs008(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	var v interface{}
	_ = yaml.Unmarshal([]byte(a.Text), &v)
	ser, _ := json.Marshal(v)
	serStr := string(ser)
	var out []model.Finding
	for i, ln := range strings.Split(a.Text, "\n") {
		if !reCommentLn.MatchString(ln) {
			continue
		}
		if strings.Contains(serStr, esc(strings.TrimSpace(ln))) {
			continue
		}
		out = append(out, model.Finding{Path: a.RelPath, Line: i + 1, Rule: "VS008",
			Severity: "error", Message: "YAML comments are not allowed in artefacts"})
	}
	return out
}

func esc(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// VS009 — file size (warning from 80000 bytes, error over 100000).
func init() { register(vs009) }
func vs009(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.NBytes <= 80000 {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	if a.NBytes > 100000 {
		s = "error"
	}
	return []model.Finding{line1(a, "VS009", s,
		fmt.Sprintf("file is %v bytes; the verifier reads at most 100000", a.NBytes))}
}

// VS010 — description lead sentence and length.
func init() { register(vs010) }
func vs010(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	desc := ""
	if a.DescIsStr {
		desc = a.Description
	}
	var out []model.Finding
	if reThisArt.MatchString(desc) {
		out = append(out, line1(a, "VS010", "error", "description starts with a lead sentence, not `This artefact...`"))
	}
	if !a.IsDoc {
		trimmed := reDescTrim.ReplaceAllString(desc, "")
		n := len(strings.Split(trimmed, "\n"))
		if n > 12 || len(desc) > 1000 {
			out = append(out, line1(a, "VS010", "error",
				fmt.Sprintf("description is %v lines / %v chars; keep it to 12 lines / 1000 chars and move the rest into doc<Name>.yml", n, len(desc))))
		}
	}
	return out
}
