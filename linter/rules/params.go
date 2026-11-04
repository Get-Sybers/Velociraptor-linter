package rules

import (
	"fmt"
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

var (
	reParamName = regexp.MustCompile(`^[A-Z]([A-Za-z0-9]|_[0-9A-Z])*$`)

	reRegexSuf     = regexp.MustCompile(`(?i)Regex$`)
	reGlobsSuf     = regexp.MustCompile(`(?i)Globs$`)
	reGlobSuf      = regexp.MustCompile(`(?i)Glob$`)
	reAfterBefore  = regexp.MustCompile(`(?i)(After|Before)$`)
	reTableList    = regexp.MustCompile(`(?i)(Table|List)$`)
	reSelectionSuf = regexp.MustCompile(`(?i)Selection$`)
	reIntSuf       = regexp.MustCompile(`(?i)(Limit|Count|Rows|Workers|Timeout|Depth)$`)
	reYaraSuf      = regexp.MustCompile(`(?i)Yara$`)
	reTimestampNm  = regexp.MustCompile(`(?i)(After|Before|Time|Timestamp|Date)$`)

	reScopeNm   = regexp.MustCompile(`(?i)(Glob|Globs|Path|Paths|Dir|Dirs|File|Files|Table)$`)
	reFilterNm  = regexp.MustCompile(`(?i)(Regex|Yara|After|Before|Time|Timestamp|Date)$`)
	reFilterPfx = regexp.MustCompile(`(?i)^(Include|Exclude|Only)[A-Z]`)
	reOutputPfx = regexp.MustCompile(`(?i)^(Upload|Max)[A-Z]`)

	rePatternSuf = regexp.MustCompile(`(?i)(Glob|Globs|Regex)$`)
	reQuote      = regexp.MustCompile(`(?i)\bquot(e|es|ed|ing)\b`)
	reExample    = regexp.MustCompile(`(?i)\be\.g\.|\bfor example\b|\bexample\b|\bsuch as\b`)
	reWildcard   = regexp.MustCompile(`(?i)wildcard|anchor|full|complete|entire|verbatim|as[- ]is`)
)

var stageNames = []string{"Selection", "Scope", "Filter", "Output", "Server"}

func allowedTypes(n string) string {
	switch {
	case reRegexSuf.MatchString(n):
		return "regex"
	case reGlobsSuf.MatchString(n):
		return "csv|json_array"
	case reGlobSuf.MatchString(n):
		return "|string"
	case reAfterBefore.MatchString(n):
		return "timestamp"
	case reTableList.MatchString(n):
		return "csv|json_array"
	case reSelectionSuf.MatchString(n):
		return "multichoice|choices|artifactset"
	case reIntSuf.MatchString(n):
		return "int|int64|integer"
	case reYaraSuf.MatchString(n):
		return "yara"
	}
	return ""
}

func stage(n, t string) int {
	switch {
	case t == "hidden" || t == "server_metadata" || t == "redacted":
		return 4
	case t == "artifactset" || reSelectionSuf.MatchString(n):
		return 0
	case reScopeNm.MatchString(n):
		return 1
	case reFilterNm.MatchString(n) || reFilterPfx.MatchString(n):
		return 2
	case t == "upload" || t == "upload_file" || reOutputPfx.MatchString(n) || reIntSuf.MatchString(n):
		return 3
	}
	return -1
}

func phint(p model.Param) string {
	if p.HasDesc() {
		return p.Description
	}
	return ""
}

// VS030–VS037, VS039 (the STRUCT-tier parameter rules). VS035 and VS038 (which
// scan the VQL text) are in paramsvql.go.
func init() { register(vsParams) }
func vsParams(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc {
		return nil
	}
	var out []model.Finding
	// R030shape: parameters present but not a list.
	if a.HasKey("parameters") && !a.ParamsIsList {
		out = append(out, line1(a, "VS030", "error", "`parameters` is a list"))
	}
	if !a.ParamsIsList {
		return out
	}
	nameCount := map[string]int{}
	for i, p := range a.Parameters {
		if !(p.IsDict && p.NameIsStr) {
			out = append(out, line1(a, "VS030", "error",
				fmt.Sprintf("parameter %v: each parameter is a mapping with a `name`", i+1)))
			continue
		}
		out = append(out, paramMsgs(a, cx, p)...)
		nameCount[p.Name]++
	}
	for name, c := range nameCount {
		if c > 1 {
			out = append(out, line1(a, "VS030", "error", fmt.Sprintf("duplicate parameter `%v`", name)))
		}
	}
	out = append(out, vs036(a)...)
	return out
}

func paramMsgs(a *model.Artefact, cx *Context, p model.Param) []model.Finding {
	var out []model.Finding
	name := p.Name
	pt := p.Type
	hint := phint(p)
	s := sev(a.Env, cx.Strict)

	if !reParamName.MatchString(name) {
		out = append(out, line1(a, "VS030", "error", fmt.Sprintf("parameter `%v` must be UpperCamelCase", name)))
	}
	if !reNonSpace.MatchString(hint) {
		out = append(out, line1(a, "VS031", "error", fmt.Sprintf("parameter `%v` needs a `description` hint", name)))
	}
	if len(hint) > 500 {
		out = append(out, line1(a, "VS037", s, fmt.Sprintf("hint for `%v` is %v characters; keep it under 500 and move the explanation to the doc companion", name, len(hint))))
	}
	if !p.Has("default") && !matchAny(pt, "hidden", "upload", "upload_file", "server_metadata", "timestamp") {
		out = append(out, line1(a, "VS032", "error", fmt.Sprintf("parameter `%v` needs a `default`", name)))
	}
	if at := allowedTypes(name); at != "" && !strings.Contains("|"+at+"|", "|"+pt+"|") {
		out = append(out, line1(a, "VS033", "error", fmt.Sprintf("parameter `%v` ends in a typed suffix, so its type is %v, not `%v`", name, at, pt)))
	}
	if pt == "regex" && !reRegexSuf.MatchString(name) {
		out = append(out, line1(a, "VS033", "error", fmt.Sprintf("parameter `%v` is type `regex`; its name ends with Regex", name)))
	}
	if pt == "timestamp" && !reTimestampNm.MatchString(name) {
		out = append(out, line1(a, "VS033", "error", fmt.Sprintf("parameter `%v` is type `timestamp`; its name ends with After/Before/Time/Timestamp/Date", name)))
	}
	if pt != strings.ToLower(pt) {
		out = append(out, line1(a, "VS033", "error", fmt.Sprintf("parameter type `%v` is lower case", pt)))
	}
	if rePatternSuf.MatchString(name) && reNonSpace.MatchString(hint) {
		kind := "regex"
		if strings.Contains(name, "Glob") {
			kind = "glob"
		}
		if !strings.Contains(strings.ToLower(hint), kind) {
			out = append(out, line1(a, "VS034", "error", fmt.Sprintf("hint for `%v` must say it is a %v and that the full pattern (wildcards/anchors included) is the input", name, kind)))
		}
		if !reWildcard.MatchString(hint) {
			out = append(out, line1(a, "VS034", s, fmt.Sprintf("hint for `%v` should state that wildcards/anchors live in the value itself (it is used verbatim)", name)))
		}
		var missing []string
		if !reQuote.MatchString(hint) {
			missing = append(missing, "whether quotes are needed")
		}
		if !reExample.MatchString(hint) {
			missing = append(missing, "an example (`e.g. ...`)")
		}
		if len(missing) > 0 {
			out = append(out, line1(a, "VS039", s, fmt.Sprintf("hint for `%v` lacks %v", name, strings.Join(missing, " and "))))
		}
	}
	return out
}

// VS036 — parameter stage order (Selection > Scope > Filter > Output > Server).
func vs036(a *model.Artefact) []model.Finding {
	var out []model.Finding
	ps := a.Parameters
	for i := range ps {
		if !ps[i].IsDict {
			continue
		}
		si := stage(ps[i].Name, ps[i].Type)
		if si < 0 {
			continue
		}
		for j := 0; j < i; j++ {
			if !ps[j].IsDict {
				continue
			}
			sj := stage(ps[j].Name, ps[j].Type)
			if sj > si {
				out = append(out, line1(a, "VS036", "error",
					fmt.Sprintf("parameter `%v` (%v) comes before `%v` (%v); order is Selection > Scope > Filter > Output > Server",
						ps[j].Name, stageNames[sj], ps[i].Name, stageNames[si])))
				break
			}
		}
	}
	return out
}

func matchAny(s string, opts ...string) bool {
	for _, o := range opts {
		if s == o {
			return true
		}
	}
	return false
}
