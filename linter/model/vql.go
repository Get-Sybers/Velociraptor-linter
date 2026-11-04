package model

import (
	"www.velocidex.com/golang/vfilter"
)

// Call is a call-site extracted from a parsed VQL block.
type Call struct {
	Type string // vfilter's classification ("plugin", "function", "symbol", ...)
	Name string
	Args []string // keyword argument names
}

// Def is a LET definition extracted from a parsed VQL block.
type Def struct {
	Name string
}

var vqlScope = vfilter.NewScope()

// VQLParses reports whether a VQL block parses.
func VQLParses(vql string) bool {
	_, err := vfilter.MultiParse(vql)
	return err == nil
}

// Calls returns the call-sites in a VQL block, via the engine's own parser.
func Calls(vql string) []Call {
	stmts, err := vfilter.MultiParse(vql)
	if err != nil {
		return nil
	}
	var out []Call
	for _, st := range stmts {
		v := vfilter.NewVisitor(vqlScope, vfilter.CollectCallSites)
		v.Visit(st)
		for _, cs := range v.CallSites {
			c := Call{Type: cs.Type, Name: cs.Name}
			for _, arg := range cs.Args {
				c.Args = append(c.Args, arg.Name)
			}
			out = append(out, c)
		}
	}
	return out
}

// Defs returns the LET definitions in a VQL block.
func Defs(vql string) []Def {
	stmts, err := vfilter.MultiParse(vql)
	if err != nil {
		return nil
	}
	var out []Def
	for _, st := range stmts {
		v := vfilter.NewVisitor(vqlScope, vfilter.CollectDefinitionSites)
		v.Visit(st)
		for _, d := range v.Definitions {
			out = append(out, Def{Name: d.Name})
		}
	}
	return out
}

// Comments returns the comments in a VQL block.
func Comments(vql string) []string {
	stmts, err := vfilter.MultiParseWithComments(vql)
	if err != nil {
		return nil
	}
	var out []string
	for _, st := range stmts {
		v := vfilter.NewVisitor(vqlScope, vfilter.FormatOptions{CollectComments: true})
		v.Visit(st)
		out = append(out, v.Comments...)
	}
	return out
}
