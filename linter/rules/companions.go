package rules

import (
	"encoding/json"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"velociraptor-linter/model"
)

var (
	reDocPath = regexp.MustCompile(`(?i)([^/]+)\.yaml$`)
	reArtPath = regexp.MustCompile(`(?i)doc([^/]+)\.yml$`)
)

// VS011 — when a doc companion exists, `reference` links it first.
func init() { register(vs011) }
func vs011(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || a.IsDoc || !a.IsYAMLExt() || !a.NameIsStr {
		return nil
	}
	docPath := reDocPath.ReplaceAllString(a.RelPath, "doc$1.yml")
	if !cx.FileSet[docPath] {
		return nil
	}
	link := "#/artifacts/" + a.Name + ".Doc"
	if strings.Contains(refSerialize(a.RefNode), link) {
		return nil
	}
	return []model.Finding{line1(a, "VS011", "error",
		"`reference` must link the doc companion: [Documentation](#/artifacts/"+a.Name+".Doc)")}
}

// VS012 — doc companion shape: its artefact exists, one notebook source, a
// markdown first cell with a template, no parameters.
func init() { register(vs012) }
func vs012(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || !a.IsDoc {
		return nil
	}
	var out []model.Finding
	artPath := reArtPath.ReplaceAllString(a.RelPath, "$1.yaml")
	if !cx.FileSet[artPath] {
		out = append(out, line1(a, "VS012", "error", "doc companion has no artefact `"+artPath+"`"))
	}
	oneSrc := len(a.Sources) == 1 && a.Sources[0].IsDict && len(a.Sources[0].Notebook) > 0
	if !oneSrc {
		out = append(out, line1(a, "VS012", "error", "doc companion has exactly one source holding `notebook` cells"))
	} else {
		cell := a.Sources[0].Notebook[0]
		if !(cell.Type == "markdown" || cell.Type == "md") {
			out = append(out, line1(a, "VS012", "error", "the first notebook cell is a markdown cell"))
		} else if !(cell.HasTemplate() && reNonSpace.MatchString(cell.Template)) {
			out = append(out, line1(a, "VS012", "error", "the markdown cell needs a `template`"))
		}
	}
	if a.ParamsIsList && len(a.Parameters) > 0 {
		out = append(out, line1(a, "VS012", "error", "doc companions take no parameters"))
	}
	return out
}

func refSerialize(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	var v interface{}
	if err := n.Decode(&v); err != nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}
