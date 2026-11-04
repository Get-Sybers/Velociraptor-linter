package rules

import (
	"fmt"
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

var (
	reVarName    = regexp.MustCompile(`^_?[A-Z][A-Za-z0-9]*$`)
	reLinterDir  = regexp.MustCompile(`^(//|--)\s*linter:\s*[a-z_]+:\S*\s*$`)
	reLogCall    = regexp.MustCompile(`\blog\(([^()]*(?:\([^()]*\)[^()]*){0,4})\)`)
	reHasLevel   = regexp.MustCompile(`\blevel\s*=`)
	reLogLevel   = regexp.MustCompile(`\blevel\s*=\s*["']([A-Za-z]+)["']`)
	reLogMsgDQ   = regexp.MustCompile(`\bmessage\s*=\s*"([^"]*)"`)
	reLogMsgSQ   = regexp.MustCompile(`\bmessage\s*=\s*'([^']*)'`)
	reKnownLevel = regexp.MustCompile(`^(DEFAULT|ERROR|WARN|WARNING|INFO|DEBUG|ALERT)$`)
	reErrText    = regexp.MustCompile(`(?i)(Error:|Symbol.+?not found|Expecting a path arg type, not|Field.+?is required|Unexpected arg)`)
	reWS         = regexp.MustCompile(`\s+`)
)

// VS023 — no VQL comments except `// linter:` directives (AST comments).
func init() { register(vs023) }
func vs023(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, c := range model.Comments(b.VQL) {
			t := strings.TrimSpace(c)
			if reLinterDir.MatchString(t) {
				continue
			}
			out = append(out, line1(a, "VS023", "error",
				fmt.Sprintf("comment in `%v`; VQL carries no comments (only `// linter: <rule>:<subject>` directives): %v", b.Field, t)))
		}
	}
	return out
}

// VS062 — LET names are UpperCamelCase (AST definition sites).
func init() { register(vs062) }
func vs062(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, d := range model.Defs(b.VQL) {
			if d.Name == "" || reVarName.MatchString(d.Name) {
				continue
			}
			out = append(out, line1(a, "VS062", sev(a.Env, cx.Strict),
				fmt.Sprintf("`LET %v` in `%v`: variables are UpperCamelCase like parameters (`_` prefix for scratch values)", d.Name, b.Field)))
		}
	}
	return out
}

// VS060/VS061 — log() level and error text. Text-based: vfilter exposes call
// and argument names but not argument values.
func init() { register(vsLog) }
func vsLog(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	var out []model.Finding
	for _, b := range a.Blocks() {
		q := reWS.ReplaceAllString(b.VQL, " ")
		for _, m := range reLogCall.FindAllStringSubmatch(q, -1) {
			body := m[1]
			if !reHasLevel.MatchString(body) {
				out = append(out, line1(a, "VS060", s,
					fmt.Sprintf("`log()` in `%v` has no `level=`; say ERROR, WARN, INFO or DEBUG (an ERROR marks the collection failed)", b.Field)))
			}
			lvl := ""
			if mm := reLogLevel.FindStringSubmatch(body); mm != nil {
				lvl = mm[1]
			}
			if lvl != "" && !reKnownLevel.MatchString(strings.ToUpper(lvl)) {
				out = append(out, line1(a, "VS060", s,
					fmt.Sprintf("`log()` level `%v` is not one Velociraptor knows (ERROR, WARN, INFO, DEBUG)", lvl)))
			}
			msg := ""
			if mm := reLogMsgDQ.FindStringSubmatch(body); mm != nil {
				msg = mm[1]
			} else if mm := reLogMsgSQ.FindStringSubmatch(body); mm != nil {
				msg = mm[1]
			}
			if lvl != "" && strings.ToUpper(lvl) != "ERROR" && msg != "" && reErrText.MatchString(msg) {
				out = append(out, line1(a, "VS061", s,
					fmt.Sprintf("`log()` in `%v` at level %v carries error text (`Error:` ...); Velociraptor marks the collection failed on it, so either log at ERROR or reword the message", b.Field, strings.ToUpper(lvl))))
			}
		}
	}
	return out
}
