package rules

import (
	"fmt"
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

var (
	reNameCharset = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
	reBadSeg      = regexp.MustCompile(`^[^A-Z]|[^A-Za-z0-9]`)
	rePlatform    = regexp.MustCompile(`^(Windows|Linux|MacOS|Generic|General|Server|Bundle|DetectRaptor)$`)
)

const platformList = "Bundle, DetectRaptor, General, Generic, Linux, MacOS, Server, Windows"

// VS003 — name is present and matches the file path.
func init() { register(vs003) }
func vs003(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	if !a.NameIsStr || a.Name == "" {
		return []model.Finding{line1(a, "VS003", "error", "`name` is required")}
	}
	if a.LayoutOK() && (a.IsDoc || a.IsYAMLExt()) && a.Name != a.ExpectedName() {
		return []model.Finding{line1(a, "VS003", "error",
			fmt.Sprintf("name `%v` does not match the file path (expected `%v`)", a.Name, a.ExpectedName()))}
	}
	return nil
}

// VS004 — name charset, UpperCamelCase segments, platform second segment, 3–6 segments.
func init() { register(vs004) }
func vs004(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || !a.NameIsStr || a.Name == "" {
		return nil
	}
	name := a.Name
	if !reNameCharset.MatchString(name) {
		return []model.Finding{line1(a, "VS004", "error",
			"name may only contain a-z, A-Z, 0-9, `_` and `.`")}
	}
	segs := strings.Split(name, ".")
	var out []model.Finding
	var bad []string
	for _, s := range segs {
		if reBadSeg.MatchString(s) {
			bad = append(bad, s)
		}
	}
	if len(bad) > 0 {
		out = append(out, line1(a, "VS004", "error",
			"name segments must be UpperCamelCase ASCII: "+strings.Join(bad, ", ")))
	}
	second := ""
	if len(segs) >= 2 {
		second = segs[1]
	}
	if !rePlatform.MatchString(second) {
		out = append(out, line1(a, "VS004", "error",
			fmt.Sprintf("second segment `%v` is not a platform: %s", second, platformList)))
	}
	if len(segs) < 3 || len(segs) > 6 {
		out = append(out, line1(a, "VS004", "error",
			"name has 3 to 6 dot-separated segments"))
	}
	return out
}

// VS042 — a name is defined once per tree (Dev or Prod).
func init() { register(vs042) }
func vs042(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc || !a.NameIsStr || a.Name == "" {
		return nil
	}
	rels := cx.ByNameEnv[a.Name+"\x00"+a.Env]
	if len(rels) <= 1 {
		return nil
	}
	return []model.Finding{line1(a, "VS042", "error",
		fmt.Sprintf("`%v` is defined more than once in %v: %v", a.Name, a.Env, strings.Join(rels, ", ")))}
}

// VS026 — name collides with an upstream artefact (built-in, Exchange, DetectRaptor).
func init() { register(vs026) }
func vs026(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || !cx.HasUniverse || !a.NameIsStr {
		return nil
	}
	if cx.inUniverse(a.Name) {
		return []model.Finding{line1(a, "VS026", "error",
			fmt.Sprintf("name `%v` collides with an upstream artefact (built-in, Exchange or DetectRaptor)", a.Name))}
	}
	return nil
}
