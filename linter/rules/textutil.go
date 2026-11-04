package rules

import (
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

// VQL text helpers for masking string/comment literals and flattening nested
// brackets. Used by the rules that inspect query text (projections, literal
// concatenation, parameter usage) where vfilter's visitor does not expose the
// needed detail.
var (
	reStrLit = regexp.MustCompile(`(?s)'{3}.*?'{3}|"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|` + "`[^`\n]*`")
	reCmtLit = regexp.MustCompile(`(?s)//[^\n]*|--[^\n]*|/\*.*?\*/`)
	reParenN = regexp.MustCompile(`\([^()]*\)`)
	reBraceN = regexp.MustCompile(`\{[^{}]*\}`)
	reBrackN = regexp.MustCompile(`\[[^\[\]]*\]`)

	reSelectBOL = regexp.MustCompile(`(?m)^SELECT\b`)
	reFromBOL   = regexp.MustCompile(`(?m)^FROM\b`)
	reFromWord  = regexp.MustCompile(`\bFROM\b`)

	reBareCol      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reAliasColEnd  = regexp.MustCompile(`(?i)\bAS\s+[A-Za-z_][A-Za-z0-9_]*\s*$`)
	reAliasNameEnd = regexp.MustCompile(`(?i)\bAS\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)

	reCallsArt   = regexp.MustCompile(`\bArtifact\.[A-Za-z0-9_.]+\s*\(`)
	reFromArt    = regexp.MustCompile(`\bFROM\s+Artifact\.`)
	reMetaCol    = regexp.MustCompile(`^_`)
	rePayloadCol = regexp.MustCompile(`(?i)Upload$|^Upload|^Content$|^Parsed$|^Raw`)
)

func noStr(q string) string { return reStrLit.ReplaceAllString(q, " ") }
func mask(q string) string  { return reCmtLit.ReplaceAllString(noStr(q), " ") }

func unnest(s string) string {
	s = reParenN.ReplaceAllString(s, "()")
	s = reBraceN.ReplaceAllString(s, "{}")
	s = reBrackN.ReplaceAllString(s, "[]")
	return s
}

func flat(s string) string {
	for i := 0; i < 6; i++ {
		s = unnest(s)
	}
	return s
}

func finalStmt(m string) string {
	parts := reSelectBOL.Split(m, -1)
	return parts[len(parts)-1]
}

func projText(f string) string {
	if reFromBOL.MatchString(f) {
		return reFromBOL.Split(f, 2)[0]
	}
	return reFromWord.Split(f, 2)[0]
}

func projItems(q string) []string {
	t := flat(projText(finalStmt(mask(q))))
	var out []string
	for _, p := range strings.Split(t, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func itemKind(item string) string {
	switch {
	case item == "*":
		return "star"
	case reBareCol.MatchString(item):
		return "bare"
	case reAliasColEnd.MatchString(item):
		return "alias"
	default:
		return "expr"
	}
}

func itemName(item string) string {
	if reBareCol.MatchString(item) {
		return item
	}
	if m := reAliasNameEnd.FindStringSubmatch(item); m != nil {
		return m[1]
	}
	return ""
}

func projNames(q string) []string {
	var out []string
	for _, item := range projItems(q) {
		if k := itemKind(item); k == "bare" || k == "alias" {
			out = append(out, itemName(item))
		}
	}
	return out
}

func hasStar(q string) bool {
	for _, item := range projItems(q) {
		if item == "*" {
			return true
		}
	}
	return false
}

// allVQL joins every VQL block (raw). allVQLAndCells also appends notebook vql
// cell templates.
func allVQL(a *model.Artefact) string {
	var parts []string
	for _, b := range a.Blocks() {
		parts = append(parts, b.VQL)
	}
	return strings.Join(parts, "\n")
}

func allVQLAndCells(a *model.Artefact) string {
	parts := []string{allVQL(a)}
	for _, s := range a.Sources {
		for _, c := range s.Notebook {
			if c.Type == "vql" && c.HasTemplate() {
				parts = append(parts, c.Template)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// definedNames returns the LET names across all blocks (for VS081/VS082).
func definedNames(a *model.Artefact) map[string]bool {
	set := map[string]bool{}
	for _, d := range model.Defs(allVQL(a)) {
		if d.Name != "" {
			set[d.Name] = true
		}
	}
	return set
}

// sourceNames returns the named sources of the artefact (for VS082).
func sourceNames(a *model.Artefact) map[string]bool {
	set := map[string]bool{}
	for _, s := range a.Sources {
		if s.NameIsStr && s.Name != "" {
			set[s.Name] = true
		}
	}
	return set
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
