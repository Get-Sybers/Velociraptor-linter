package rules

import (
	"fmt"

	"velociraptor-linter/model"
)

// VS085 — a VQL block that does not parse. The AST rules only inspect blocks the
// engine can parse, so without this a syntax error would surface only under
// `verify`; this reports it under the linter too, naming the exact block.
func init() { register(vs085) }
func vs085(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		if !model.VQLParses(b.VQL) {
			out = append(out, line1(a, "VS085", "error",
				fmt.Sprintf("VQL in `%v` does not parse; run `make verify` for the parser error", b.Field)))
		}
	}
	return out
}
