package rules

import (
	"fmt"
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

var (
	reFromBare   = regexp.MustCompile(`\bFROM\s+([A-Za-z_][A-Za-z0-9_]*)(\s*[(.]?)`)
	reYaraHit    = regexp.MustCompile(`/%[vs]/|\$[A-Za-z0-9_]+\s*=\s*/["']\s*\+`)
	reYaraEscape = regexp.MustCompile(`re\s*=\s*["']/["']`)
	reLetPlugin  = regexp.MustCompile(`\bLET\s+[A-Za-z_][A-Za-z0-9_]*\s*<?=\s*([a-z_][a-z0-9_]*)\s*\(`)
	reFromPlugin = regexp.MustCompile(`\bFROM\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// VS080 — a plugin assigned or used as a value (`LET X <= execve(...)`).
func init() { register(vs080) }
func vs080(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || cx.Ref == nil || !cx.Ref.Have {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, mm := range reLetPlugin.FindAllStringSubmatch(mask(b.VQL), -1) {
			n := mm[1]
			if cx.Ref.Plugins[n] && !cx.Ref.Functions[n] {
				out = append(out, line1(a, "VS080", "error",
					fmt.Sprintf("`%v()` in `%v` is a plugin, not a function: it yields rows to `SELECT ... FROM %v()` and cannot be assigned or used as a value", n, b.Field, n)))
			}
		}
	}
	return out
}

// VS081 — `FROM name()` is not a plugin of the pinned release nor a LET here.
func init() { register(vs081) }
func vs081(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || cx.Ref == nil || !cx.Ref.Have || a.HasKey("imports") {
		return nil
	}
	defs := definedNames(a)
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, mm := range reFromPlugin.FindAllStringSubmatch(mask(b.VQL), -1) {
			n := mm[1]
			if !cx.Ref.Plugins[n] && !defs[n] {
				out = append(out, line1(a, "VS081", "error",
					fmt.Sprintf("`FROM %v()` in `%v`: no such plugin in the pinned Velociraptor and no `LET %v` in this artefact", n, b.Field, n)))
			}
		}
	}
	return out
}

// VS082 — a sibling source read as a FROM variable.
func init() { register(vs082) }
func vs082(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	srcs := sourceNames(a)
	if len(srcs) == 0 {
		return nil
	}
	defs := definedNames(a)
	var out []model.Finding
	for _, b := range a.Blocks() {
		m := mask(b.VQL)
		for _, mm := range reFromBare.FindAllStringSubmatch(m, -1) {
			n, next := mm[1], mm[2]
			if strings.ContainsAny(next, "(.") {
				continue
			}
			if srcs[n] && !defs[n] {
				out = append(out, line1(a, "VS082", "error",
					fmt.Sprintf("`FROM %v` in `%v` reads source `%v` as a variable; sources are not in scope at collection time (union them in a notebook with `source(source=...)`, or call `Artifact.%v(source=...)`)", n, b.Field, n, a.Name)))
			}
		}
	}
	return out
}

// VS084 — a value placed between the `/` delimiters of a YARA regex.
func init() { register(vs084) }
func vs084(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || reYaraEscape.MatchString(allVQL(a)) {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, hit := range reYaraHit.FindAllString(b.VQL, -1) {
			out = append(out, line1(a, "VS084", "error",
				fmt.Sprintf("`%v` in `%v` puts a value between the `/` delimiters of a YARA regex; escape `/` in the value first (`regex_replace(source=..., re=\"/\", replace=\"\\\\/\")`), or a value holding `/` ends the pattern early and the rule fails to compile", hit, b.Field)))
		}
	}
	return out
}
