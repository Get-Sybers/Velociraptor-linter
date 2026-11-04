package rules

import (
	"fmt"
	"regexp"
	"strings"

	"velociraptor-linter/model"
)

func isBundle(name string) bool {
	segs := strings.Split(name, ".")
	return len(segs) >= 2 && segs[1] == "Bundle"
}

type srcProj struct {
	I        int
	Names    []string
	Star     bool
	CallsArt bool
	FromArt  bool
}

func srcProjections(a *model.Artefact) []srcProj {
	var out []srcProj
	for i, s := range a.Sources {
		if !s.QueryIsStr || s.Query == "" {
			continue
		}
		out = append(out, srcProj{
			I:        i,
			Names:    projNames(s.Query),
			Star:     hasStar(s.Query),
			CallsArt: reCallsArt.MatchString(mask(s.Query)),
			FromArt:  reFromArt.MatchString(finalStmt(mask(s.Query))),
		})
	}
	return out
}

func outArts(a *model.Artefact) bool {
	return a.ParsedOK && !a.IsDoc && typeU(a) != "NOTEBOOK" && a.SourcesIsList && a.NameIsStr
}

// VS050 — output column names are UpperCamelCase.
func init() { register(vs050) }
func vs050(a *model.Artefact, cx *Context) []model.Finding {
	if !outArts(a) {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	var out []model.Finding
	for i, src := range a.Sources {
		if !src.QueryIsStr || src.Query == "" {
			continue
		}
		for _, item := range projItems(src.Query) {
			k := itemKind(item)
			if k != "bare" && k != "alias" {
				continue
			}
			n := itemName(item)
			if reVarName.MatchString(n) {
				continue
			}
			verb := "alias"
			if k == "alias" {
				verb = "rename"
			}
			out = append(out, line1(a, "VS050", s,
				fmt.Sprintf("output column `%v` in source %v: %v it as UpperCamelCase (an all-caps acronym or a leading `_` for metadata is fine)", n, i+1, verb)))
		}
	}
	return out
}

// VS051 — wrapper/bundle/multi-source sources project `_Source`.
func init() { register(vs051) }
func vs051(a *model.Artefact, cx *Context) []model.Finding {
	if !outArts(a) {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	multi := len(a.Sources) > 1
	bundle := isBundle(a.Name)
	var out []model.Finding
	for _, sp := range srcProjections(a) {
		if !(bundle || multi || sp.CallsArt) {
			continue
		}
		if contains(sp.Names, "_Source") {
			continue
		}
		if sp.Star && sp.FromArt {
			continue
		}
		kind := "a wrapper"
		if bundle {
			kind = "a bundle"
		} else if multi {
			kind = "a multi-source"
		}
		out = append(out, line1(a, "VS051", s,
			fmt.Sprintf("source %v of %v artefact does not project `_Source` (`\\x22<name>\\x22 AS _Source`)", sp.I+1, kind)))
	}
	return out
}

// VS052 — every source of a bundle projects the same columns in the same order.
func init() { register(vs052) }
func vs052(a *model.Artefact, cx *Context) []model.Finding {
	if !outArts(a) || !isBundle(a.Name) {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	sps := srcProjections(a)
	firstIdx := -1
	var firstNames []string
	for _, sp := range sps {
		if !sp.Star && len(sp.Names) > 0 {
			firstIdx = sp.I
			firstNames = sp.Names
			break
		}
	}
	if firstIdx < 0 {
		return nil
	}
	var out []model.Finding
	for _, sp := range sps {
		if sp.I <= firstIdx || sp.Star || len(sp.Names) == 0 {
			continue
		}
		if strings.Join(sp.Names, ",") == strings.Join(firstNames, ",") {
			continue
		}
		missing := minus(firstNames, sp.Names)
		extra := minus(sp.Names, firstNames)
		p1, p2, p3 := "", "", ""
		if len(missing) > 0 {
			p1 = "missing " + strings.Join(missing, ", ")
		}
		if len(extra) > 0 {
			p2 = "extra " + strings.Join(extra, ", ")
		}
		if len(missing) == 0 && len(extra) == 0 {
			p3 = "same columns in a different order"
		}
		detail := strings.Join([]string{p1, p2, p3}, "; ")
		out = append(out, line1(a, "VS052", s,
			fmt.Sprintf("bundle source %v projects different columns from source %v (%v); every source of a bundle projects the same columns in the same order", sp.I+1, firstIdx+1, detail)))
	}
	return out
}

// VS053 — metadata columns first, payload columns last.
func init() { register(vs053) }
func vs053(a *model.Artefact, cx *Context) []model.Finding {
	if !outArts(a) {
		return nil
	}
	s := sev(a.Env, cx.Strict)
	var out []model.Finding
	for _, sp := range srcProjections(a) {
		if sp.Star || len(sp.Names) == 0 {
			continue
		}
		names := sp.Names
		meta := filterRe(names, reMetaCol)
		if len(meta) > 0 {
			head := names[:len(meta)]
			if countRe(head, reMetaCol) != len(meta) {
				out = append(out, line1(a, "VS053", s,
					fmt.Sprintf("source %v: metadata columns (`_*`) come first in the projection", sp.I+1)))
			}
		}
		payload := filterRe(names, rePayloadCol)
		if len(payload) > 0 {
			tail := names[len(names)-len(payload):]
			if countRe(tail, rePayloadCol) != len(payload) {
				out = append(out, line1(a, "VS053", s,
					fmt.Sprintf("source %v: payload columns (`Upload`, `Content`, `Parsed`, `Raw*`) come last in the projection", sp.I+1)))
			}
		}
	}
	return out
}

func minus(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range b {
		set[x] = true
	}
	var out []string
	for _, x := range a {
		if !set[x] {
			out = append(out, x)
		}
	}
	return out
}

func filterRe(xs []string, re *regexp.Regexp) []string {
	var out []string
	for _, x := range xs {
		if re.MatchString(x) {
			out = append(out, x)
		}
	}
	return out
}

func countRe(xs []string, re *regexp.Regexp) int {
	n := 0
	for _, x := range xs {
		if re.MatchString(x) {
			n++
		}
	}
	return n
}
