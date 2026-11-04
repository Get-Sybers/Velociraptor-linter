package rules

import (
	"fmt"
	"strings"

	"velociraptor-linter/model"
)

// VS024/VS025/VS040/VS083 — the shared Artifact.X() call-site walk. Uses the
// vfilter AST for call names and keyword-argument names, plus the loaded
// universe and own-tree index for resolution.
func init() { register(vsCalls) }
func vsCalls(a *model.Artefact, cx *Context) []model.Finding {
	if !a.ParsedOK || !cx.HasUniverse {
		return nil
	}
	var out []model.Finding
	for _, b := range a.Blocks() {
		for _, c := range model.Calls(b.VQL) {
			if !strings.HasPrefix(c.Name, "Artifact.") {
				continue
			}
			target := strings.TrimPrefix(c.Name, "Artifact.")
			known := cx.known(target)

			// VS024 — call does not resolve.
			if !known {
				out = append(out, line1(a, "VS024", "error",
					fmt.Sprintf("`Artifact.%v` in `%v` is not defined in this tree, the built-ins, the Exchange or DetectRaptor", target, b.Field)))
			}

			// VS040 — Prod artefact depends on a Dev-only artefact.
			if a.Env == "Prod" && !cx.inUniverse(target) && cx.inOwn(target) && !cx.ownHasEnv(target, "Prod") {
				out = append(out, line1(a, "VS040", "error",
					fmt.Sprintf("Prod artefact depends on `%v` which exists only in Dev", target)))
			}

			// VS025 — keyword passed is not one of the callee's parameters.
			if known {
				params := cx.knownParams(target)
				for _, kw := range c.Args {
					if kw == "" || kw == "source" || kw == "preconditions" {
						continue
					}
					if !params[kw] {
						out = append(out, line1(a, "VS025", "error",
							fmt.Sprintf("`Artifact.%v` has no parameter `%v`", target, kw)))
					}
				}
			}

			// VS083 — calling an artefact whose sources are differently gated.
			if known && cx.gatesOf(target) > 1 && !hasArg(c.Args, "preconditions") && !hasArg(c.Args, "source") {
				out = append(out, line1(a, "VS083", sev(a.Env, cx.Strict),
					fmt.Sprintf("`Artifact.%v()` in `%v` has %v differently gated sources, which all run when called this way; pass `preconditions=TRUE`, or `source=` for one of them", target, b.Field, cx.gatesOf(target))))
			}
		}
	}
	return out
}

func hasArg(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}
