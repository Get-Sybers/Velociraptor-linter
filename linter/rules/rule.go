// Package rules holds the lint rules. Each rule registers itself via init();
// the CLI runs model.Load on every artefact and applies All() to each.
package rules

import "velociraptor-linter/model"

// UDef is a universe entry: an upstream artefact's parameters and gate count.
type UDef struct {
	Params []string
	Gates  int
}

// OwnDef aggregates an artefact defined in the linted tree(s).
type OwnDef struct {
	Envs   map[string]bool
	Params []string
	Gates  int
}

// Reference is the pinned release's VQL plugin/function registry.
type Reference struct {
	Have      bool
	Plugins   map[string]bool
	Functions map[string]bool
}

// Context carries cross-rule state built once per run.
type Context struct {
	Strict      bool
	FileSet     map[string]bool     // every artefact RelPath present
	ByNameEnv   map[string][]string // "Name\x00Env" -> sorted RelPaths (non-doc, named)
	Universe    map[string]UDef     // built-ins + Exchange + DetectRaptor
	HasUniverse bool                // whether the universe index was supplied
	Own         map[string]OwnDef   // artefacts defined in the linted tree
	Ref         *Reference          // VQL reference (nil-safe via Have)
}

func (cx *Context) inUniverse(t string) bool { _, ok := cx.Universe[t]; return ok }
func (cx *Context) inOwn(t string) bool      { _, ok := cx.Own[t]; return ok }
func (cx *Context) known(t string) bool      { return cx.inUniverse(t) || cx.inOwn(t) }

func (cx *Context) ownHasEnv(t, env string) bool {
	o, ok := cx.Own[t]
	return ok && o.Envs[env]
}

func (cx *Context) gatesOf(t string) int {
	if u, ok := cx.Universe[t]; ok && u.Gates != 0 {
		return u.Gates
	}
	if o, ok := cx.Own[t]; ok && o.Gates != 0 {
		return o.Gates
	}
	return 0
}

func (cx *Context) knownParams(t string) map[string]bool {
	set := map[string]bool{}
	if u, ok := cx.Universe[t]; ok {
		for _, p := range u.Params {
			set[p] = true
		}
	}
	if o, ok := cx.Own[t]; ok {
		for _, p := range o.Params {
			set[p] = true
		}
	}
	return set
}

// Rule inspects one artefact and returns findings.
type Rule func(a *model.Artefact, cx *Context) []model.Finding

var registry []Rule

func register(r Rule) { registry = append(registry, r) }

// All returns every registered rule.
func All() []Rule { return registry }

// sev is the Dev/Prod severity gate: warning in Dev, error in Prod or --strict.
func sev(env string, strict bool) string {
	if strict || env == "Prod" {
		return "error"
	}
	return "warning"
}

// line1 builds a finding anchored at line 1 (as most rules report).
func line1(a *model.Artefact, rule, severity, msg string) model.Finding {
	return model.Finding{Path: a.RelPath, Line: 1, Rule: rule, Severity: severity, Message: msg}
}
