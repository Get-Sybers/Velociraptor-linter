// Package model loads a Velociraptor artefact file and exposes the parsed
// structure, on-disk metadata, and VQL blocks that the lint rules inspect.
package model

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Finding is one linter result. Its String form is the line the CLI prints and
// sorts, identical to the legacy lint.sh format: "path:line: RULE sev: message".
type Finding struct {
	Path     string
	Line     int
	Rule     string
	Severity string // "error" | "warning"
	Message  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s %s: %s", f.Path, f.Line, f.Rule, f.Severity, f.Message)
}

// Param is one entry of an artefact's `parameters:` list.
type Param struct {
	Name        string
	NameIsStr   bool
	Type        string
	Description string
	hasDesc     bool
	IsDict      bool
	keys        map[string]bool
	Node        *yaml.Node
}

// Has reports whether the parameter mapping declared key k (even if empty).
func (p *Param) Has(k string) bool { return p.keys[k] }

// HasDesc reports whether `description` was a string.
func (p *Param) HasDesc() bool { return p.hasDesc }

// NotebookCell is one cell of a source's `notebook:` list.
type NotebookCell struct {
	Type     string
	Name     string
	Template string
	hasTmpl  bool
}

func (c *NotebookCell) HasTemplate() bool { return c.hasTmpl }

// Source is one entry of an artefact's `sources:` list.
type Source struct {
	Name         string
	NameIsStr    bool
	Query        string
	QueryIsStr   bool
	Precondition string
	PrecondIsStr bool
	Notebook     []NotebookCell
	IsDict       bool
	keys         map[string]bool
	Node         *yaml.Node
}

func (s *Source) Has(k string) bool { return s.keys[k] }

// Block is one VQL-bearing region of the artefact, labelled as the legacy
// linter labels it ("export", "precondition", "sources[0].query", ...).
type Block struct {
	Field string
	VQL   string
}

// Artefact is a parsed artefact file plus its on-disk metadata. Fields are
// populated best-effort: ParsedOK is false when the YAML did not parse or its
// top level is not a mapping, and rules must guard on it.
type Artefact struct {
	RelPath string   // path relative to the lint root, forward slashes
	Base    string   // basename
	Comps   []string // RelPath split on "/"
	Env     string   // Comps[0] if present
	Kind    string   // Comps[1] if present
	IsDoc   bool     // basename matches ^doc.*\.yml$ (case-sensitive)
	Text    string   // raw file contents
	NBytes  int

	ParsedOK bool

	Name          string
	NameIsStr     bool
	Author        string
	AuthorIsStr   bool
	Description   string
	DescIsStr     bool
	Type          string
	TypeIsStr     bool
	Export        string
	Precondition  string
	PrecondIsStr  bool
	Imports       []string
	Parameters    []Param
	ParamsIsList  bool
	Sources       []Source
	SourcesIsList bool

	topKeys map[string]bool
	RefNode *yaml.Node // the `reference` value node (nil if absent)
	root    *yaml.Node
}

// HasKey reports whether the top-level mapping declared key k.
func (a *Artefact) HasKey(k string) bool { return a.topKeys[k] }

// RefIsList reports whether `reference` is a sequence.
func (a *Artefact) RefIsList() bool {
	return a.RefNode != nil && a.RefNode.Kind == yaml.SequenceNode
}

var (
	reDocName  = regexp.MustCompile(`^doc.*\.yml$`)
	reYamlExt  = regexp.MustCompile(`(?i)\.yaml$`)
	reYmlExt   = regexp.MustCompile(`(?i)\.yml$`)
	reDocStrip = regexp.MustCompile(`(?i)^doc|\.yml$`)
)

// Load reads the artefact at absPath and records its path metadata relative to
// root. It never returns an error for a malformed artefact; it sets ParsedOK.
func Load(absPath, root string) (*Artefact, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		rel = absPath
	}
	rel = filepath.ToSlash(rel)
	a := &Artefact{
		RelPath: rel,
		Base:    filepath.Base(rel),
		Comps:   strings.Split(rel, "/"),
		Text:    string(data),
		NBytes:  len(data),
		topKeys: map[string]bool{},
	}
	if len(a.Comps) > 0 {
		a.Env = a.Comps[0]
	}
	if len(a.Comps) > 1 {
		a.Kind = a.Comps[1]
	}
	a.IsDoc = reDocName.MatchString(a.Base)

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return a, nil // ParsedOK stays false
	}
	var top *yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		top = doc.Content[0]
	} else if doc.Kind == yaml.MappingNode {
		top = &doc
	}
	if top == nil || top.Kind != yaml.MappingNode {
		return a, nil // not a mapping -> ParsedOK false (VS001)
	}
	a.ParsedOK = true
	a.root = top
	keys, vals := mapping(top)
	a.topKeys = keys
	a.Name, a.NameIsStr = scalar(vals["name"])
	a.Author, a.AuthorIsStr = scalar(vals["author"])
	a.Description, a.DescIsStr = scalar(vals["description"])
	a.Type, a.TypeIsStr = scalar(vals["type"])
	a.Export, _ = scalar(vals["export"])
	a.Precondition, a.PrecondIsStr = scalar(vals["precondition"])
	a.RefNode = vals["reference"]
	if n := vals["imports"]; n != nil && n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if s, ok := scalar(c); ok {
				a.Imports = append(a.Imports, s)
			}
		}
	}
	if n := vals["parameters"]; n != nil {
		a.ParamsIsList = n.Kind == yaml.SequenceNode
		if a.ParamsIsList {
			for _, pn := range n.Content {
				a.Parameters = append(a.Parameters, parseParam(pn))
			}
		}
	}
	if n := vals["sources"]; n != nil {
		a.SourcesIsList = n.Kind == yaml.SequenceNode
		if a.SourcesIsList {
			for _, sn := range n.Content {
				a.Sources = append(a.Sources, parseSource(sn))
			}
		}
	}
	return a, nil
}

func parseParam(n *yaml.Node) Param {
	p := Param{keys: map[string]bool{}, Node: n}
	if n.Kind != yaml.MappingNode {
		return p
	}
	p.IsDict = true
	keys, vals := mapping(n)
	p.keys = keys
	p.Name, p.NameIsStr = scalar(vals["name"])
	p.Type, _ = scalar(vals["type"])
	p.Description, p.hasDesc = scalar(vals["description"])
	return p
}

func parseSource(n *yaml.Node) Source {
	s := Source{keys: map[string]bool{}, Node: n}
	s.IsDict = n.Kind == yaml.MappingNode
	if !s.IsDict {
		return s
	}
	keys, vals := mapping(n)
	s.keys = keys
	s.Name, s.NameIsStr = scalar(vals["name"])
	s.Query, s.QueryIsStr = scalar(vals["query"])
	s.Precondition, s.PrecondIsStr = scalar(vals["precondition"])
	if nb := vals["notebook"]; nb != nil && nb.Kind == yaml.SequenceNode {
		for _, cn := range nb.Content {
			if cn.Kind != yaml.MappingNode {
				s.Notebook = append(s.Notebook, NotebookCell{})
				continue
			}
			_, cv := mapping(cn)
			cell := NotebookCell{}
			cell.Type, _ = scalar(cv["type"])
			cell.Name, _ = scalar(cv["name"])
			cell.Template, cell.hasTmpl = scalar(cv["template"])
			s.Notebook = append(s.Notebook, cell)
		}
	}
	return s
}

// Blocks returns every VQL-bearing region, labelled as the legacy linter does.
func (a *Artefact) Blocks() []Block {
	var bs []Block
	if a.Precondition != "" {
		bs = append(bs, Block{"precondition", a.Precondition})
	}
	if a.Export != "" {
		bs = append(bs, Block{"export", a.Export})
	}
	for i, s := range a.Sources {
		if s.Query != "" {
			bs = append(bs, Block{fmt.Sprintf("sources[%d].query", i), s.Query})
		}
		if s.Precondition != "" {
			bs = append(bs, Block{fmt.Sprintf("sources[%d].precondition", i), s.Precondition})
		}
	}
	return bs
}

// LayoutOK reports the Dev|Prod / Client|Server|Notebooks layout.
func (a *Artefact) LayoutOK() bool {
	if len(a.Comps) < 3 {
		return false
	}
	env := a.Comps[0]
	kind := a.Comps[1]
	return (env == "Dev" || env == "Prod") &&
		(kind == "Client" || kind == "Server" || kind == "Notebooks")
}

// ExpectedName is the artefact name implied by the file path.
func (a *Artefact) ExpectedName() string {
	stem := a.stem()
	if len(a.Comps) > 3 {
		return strings.Join(a.Comps[2:len(a.Comps)-1], ".") + "." + stem
	}
	return stem
}

func (a *Artefact) stem() string {
	if a.IsDoc {
		return reDocStrip.ReplaceAllString(a.Base, "") + ".Doc"
	}
	return reYamlExt.ReplaceAllString(a.Base, "")
}

// IsYAMLExt / IsYMLExt report the file extension (case-insensitive).
func (a *Artefact) IsYAMLExt() bool { return reYamlExt.MatchString(a.Base) }
func (a *Artefact) IsYMLExt() bool  { return reYmlExt.MatchString(a.Base) }

func mapping(n *yaml.Node) (map[string]bool, map[string]*yaml.Node) {
	keys := map[string]bool{}
	vals := map[string]*yaml.Node{}
	if n == nil || n.Kind != yaml.MappingNode {
		return keys, vals
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		keys[k] = true
		vals[k] = n.Content[i+1]
	}
	return keys, vals
}

func scalar(n *yaml.Node) (string, bool) {
	// The bool tracks whether the scalar is actually a string. A non-string
	// YAML scalar (int, bool, null) must not satisfy the string-only field
	// checks: the engine's own parse_yaml types it rather than stringifying it.
	if n != nil && n.Kind == yaml.ScalarNode {
		return n.Value, n.ShortTag() == "!!str"
	}
	return "", false
}
