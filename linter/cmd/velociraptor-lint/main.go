// velociraptor-lint runs the artefact standard over a tree of artefacts.
// It is the Go replacement for lint/Artefacts.yaml + scripts/lint.sh and keeps
// the same CLI and output so the Makefile/CI integration is drop-in.
//
//	velociraptor-lint [--strict] [--root DIR] [--universe F] [--reference F] [path ...]
//
// Default paths: Dev Prod. --universe is the definitions index (JSONL emitted by
// the pinned binary); --reference is docs/references/vql.yaml.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"velociraptor-linter/model"
	"velociraptor-linter/rules"
)

func main() {
	root := flag.String("root", ".", "repository root; paths are reported relative to it")
	strict := flag.Bool("strict", false, "treat warnings as errors everywhere")
	expect := flag.String("expect", "", "compare findings against this file; exit 1 if they differ")
	update := flag.String("update-expected", "", "rewrite this file with the findings and exit")
	universe := flag.String("universe", "", "definitions index (JSONL) for resolution rules")
	reference := flag.String("reference", "", "VQL reference (docs/references/vql.yaml) for the improper-method rules")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"Dev", "Prod"}
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fatal(err)
	}
	files, err := discover(absRoot, paths)
	if err != nil {
		fatal(err)
	}

	// Pass 1: load every artefact; index the corpus and the own-tree.
	arts := make([]*model.Artefact, 0, len(files))
	fileset := map[string]bool{}
	byNameEnv := map[string][]string{}
	own := map[string]rules.OwnDef{}
	for _, fpath := range files {
		a, err := model.Load(fpath, absRoot)
		if err != nil {
			fatal(err)
		}
		arts = append(arts, a)
		fileset[a.RelPath] = true
		if !a.IsDoc && a.NameIsStr && a.Name != "" {
			k := a.Name + "\x00" + a.Env
			byNameEnv[k] = append(byNameEnv[k], a.RelPath)
			d := own[a.Name]
			if d.Envs == nil {
				d.Envs = map[string]bool{}
			}
			d.Envs[a.Env] = true
			d.Params = append(d.Params, paramNames(a)...)
			if g := gateCount(a); g > d.Gates {
				d.Gates = g
			}
			own[a.Name] = d
		}
	}
	for k := range byNameEnv {
		sort.Strings(byNameEnv[k])
	}

	cx := &rules.Context{
		Strict:      *strict,
		FileSet:     fileset,
		ByNameEnv:   byNameEnv,
		Own:         own,
		Universe:    loadUniverse(*universe),
		HasUniverse: *universe != "",
		Ref:         loadReference(*reference),
	}

	// Pass 2: run every rule over every artefact.
	var findings []model.Finding
	for _, a := range arts {
		for _, r := range rules.All() {
			findings = append(findings, r(a, cx)...)
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].String() < findings[j].String() })
	var b strings.Builder
	for _, fd := range findings {
		b.WriteString(fd.String())
		b.WriteByte('\n')
	}
	report := b.String()

	switch {
	case *update != "":
		if err := os.WriteFile(*update, []byte(report), 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("velociraptor-lint: wrote %d expected findings to %s\n", len(findings), *update)
		return
	case *expect != "":
		want, err := os.ReadFile(*expect)
		if err != nil {
			fatal(err)
		}
		if string(want) == report {
			fmt.Printf("lint-test: %d findings as expected\n", len(findings))
			return
		}
		fmt.Fprintf(os.Stderr, "lint-test: findings differ from %s (run with --update-expected if the change is intended)\n", *expect)
		fmt.Fprint(os.Stderr, diff(string(want), report))
		os.Exit(1)
	}

	errors, warnings := 0, 0
	for _, fd := range findings {
		switch fd.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}
	gh := os.Getenv("GITHUB_ACTIONS") == "true"
	for _, fd := range findings {
		if gh {
			fmt.Printf("::%s file=%s,line=%d,title=%s::%s\n", fd.Severity, fd.Path, fd.Line, fd.Rule, fd.Message)
		} else {
			fmt.Println(fd.String())
		}
	}
	fmt.Printf("lint: %d errors, %d warnings\n", errors, warnings)
	if errors > 0 {
		os.Exit(1)
	}
}

func paramNames(a *model.Artefact) []string {
	var out []string
	for _, p := range a.Parameters {
		if p.NameIsStr && p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out
}

// gateCount is the number of distinct non-empty source-level preconditions.
func gateCount(a *model.Artefact) int {
	seen := map[string]bool{}
	for _, s := range a.Sources {
		if s.PrecondIsStr && s.Precondition != "" {
			seen[s.Precondition] = true
		}
	}
	return len(seen)
}

func loadUniverse(path string) map[string]rules.UDef {
	u := map[string]rules.UDef{}
	if path == "" {
		return u
	}
	f, err := os.Open(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 32*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row struct {
			Name    string
			Params  []string
			Aliases []string
			Gates   int
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		d := rules.UDef{Params: row.Params, Gates: row.Gates}
		u[row.Name] = d
		for _, al := range row.Aliases {
			u[al] = d
		}
	}
	return u
}

func loadReference(path string) *rules.Reference {
	r := &rules.Reference{Plugins: map[string]bool{}, Functions: map[string]bool{}}
	if path == "" {
		return r
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	var rows []struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &rows); err != nil {
		fatal(err)
	}
	for _, row := range rows {
		switch row.Type {
		case "Plugin":
			r.Plugins[row.Name] = true
		case "Function":
			r.Functions[row.Name] = true
		}
	}
	r.Have = len(rows) > 0
	return r
}

func discover(root string, paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		base := filepath.Join(root, filepath.FromSlash(p))
		info, err := os.Stat(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, base)
			continue
		}
		err = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if name := d.Name(); strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func diff(want, got string) string {
	wl := strings.Split(strings.TrimRight(want, "\n"), "\n")
	gl := strings.Split(strings.TrimRight(got, "\n"), "\n")
	wset := map[string]bool{}
	for _, l := range wl {
		wset[l] = true
	}
	gset := map[string]bool{}
	for _, l := range gl {
		gset[l] = true
	}
	var b strings.Builder
	for _, l := range wl {
		if !gset[l] {
			b.WriteString("- " + l + "\n")
		}
	}
	for _, l := range gl {
		if !wset[l] {
			b.WriteString("+ " + l + "\n")
		}
	}
	return b.String()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "velociraptor-lint:", err)
	os.Exit(2)
}
