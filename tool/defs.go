package main

// Go-to-definition by name: a lightweight index of where names are defined across the
// repo, built with per-language patterns. Rougher than a language server, but works for
// any language and needs nothing installed.

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Def struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Kind string `json:"kind,omitempty"`
}

var (
	defJS = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(?:function\*?|class|interface|type|enum|namespace)\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s+(?:(?:public|private|protected|static|readonly|override|async|get|set)\s+)*([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\([^)]*\)\s*(?::[^={]+)?\{`),
	}
	defPHP = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:(?:abstract|final|readonly)\s+)*(?:class|interface|trait|enum)\s+(\w+)`),
		regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|final|abstract)\s+)*function\s+&?(\w+)\s*\(`),
		regexp.MustCompile(`^\s*(?:(?:public|private|protected|final)\s+)*const\s+(\w+)\s*=`),
	}
	defGo = []*regexp.Regexp{
		regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?(\w+)\s*[\[(]`),
		regexp.MustCompile(`^(?:type|var|const)\s+(\w+)`),
		regexp.MustCompile(`^\t(\w+)\s+(?:struct|interface|func|=|[\w.*\[\]]+\s*(?:=|$))`),
	}
	defPy = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)`),
		regexp.MustCompile(`^\s*class\s+(\w+)`),
		regexp.MustCompile(`^([A-Za-z_]\w*)\s*(?::[^=]+)?=[^=]`),
	}
	defGD = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:static\s+)?func\s+(\w+)`),
		regexp.MustCompile(`^(?:class_name|class|signal|enum)\s+(\w+)`),
		regexp.MustCompile(`^(?:@\w+(?:\([^)]*\))?\s+)*(?:var|const)\s+(\w+)`),
	}
	// Anything else (Java, C#, C/C++, Rust, Kotlin, Swift, Lua…): common declaration shapes.
	defGeneric = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:class|struct|interface|enum|trait|record|object|protocol|union|module|namespace)\s+([A-Za-z_]\w*)`),
		regexp.MustCompile(`\b(?:fn|func|fun|function|def|sub|proc)\s+([A-Za-z_]\w*)`),
		regexp.MustCompile(`^\s*(?:typedef\s+.*\s+|#define\s+)([A-Za-z_]\w*)`),
	}
)

var defKeywordsSkip = set("if", "for", "while", "switch", "catch", "return", "function", "else", "do", "try", "new", "await", "typeof", "constructor", "super", "this", "get", "set")

func defPatterns(ext string) []*regexp.Regexp {
	switch langByExt[ext] {
	case "js":
		return defJS
	case "php":
		return defPHP
	case "go":
		return defGo
	case "py":
		return defPy
	}
	if ext == "gd" {
		return defGD
	}
	if codeExt[ext] {
		return defGeneric
	}
	return nil
}

type defIndex struct {
	mu    sync.Mutex
	at    time.Time
	names map[string][]Def
}

var defs = &defIndex{}

func (d *defIndex) get(files []string) map[string][]Def {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.names != nil && time.Since(d.at) < 20*time.Second {
		return d.names
	}
	names := map[string][]Def{}
	for _, f := range files {
		pats := defPatterns(extOf(f))
		if pats == nil {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(f))
		if st, err := os.Stat(full); err != nil || st.Size() > 1<<20 {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		for i, ln := range strings.Split(string(b), "\n") {
			if len(ln) > 400 {
				continue
			}
			for _, re := range pats {
				if m := re.FindStringSubmatch(ln); m != nil && len(m[1]) > 1 && !defKeywordsSkip[m[1]] {
					names[m[1]] = append(names[m[1]], Def{File: f, Line: i + 1})
					break
				}
			}
		}
	}
	d.names, d.at = names, time.Now()
	return names
}

// resolvedImport is where a name imported into a file is defined: a file and line, or a
// folder (a Go package) to look inside. Shared by the server and static exports.
type resolvedImport struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	Dir  bool   `json:"dir,omitempty"`
}

// resolveImports turns a file's imports into definition targets. Imported names that point at
// a file are resolved to a line now (resolveSymbol reads the target), so callers don't need to.
func resolveImports(c *repoCtx, fl *fileLinks) map[string]resolvedImport {
	if len(fl.Imports) == 0 {
		return nil
	}
	out := map[string]resolvedImport{}
	for name, t := range fl.Imports {
		if c.dirs[t.Path] || t.Path == "." {
			out[name] = resolvedImport{Path: t.Path, Dir: true}
			continue
		}
		sym := t.Symbol
		if sym == "" {
			sym = name
		}
		out[name] = resolvedImport{Path: t.Path, Line: max(1, resolveSymbol(t.Path, sym))}
	}
	return out
}

// findDefinition: where name, used in file from, is defined.
func findDefinition(c *repoCtx, from, name string) []Def {
	var imports map[string]resolvedImport
	var links []Link
	if from != "" && c.set[from] {
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from))); err == nil {
			fl := linksFor(c, from, string(b))
			imports, links = resolveImports(c, fl), fl.Links
		}
	}
	return rankDefinitions(name, defs.get(c.files)[name], from, imports, links)
}

// rankDefinitions picks and orders candidate definitions: the file's own import of the name
// first (inside the package, for a Go import), then a definition in the same file, then
// every definition ranked by closeness (folders the file imports from, then its own folder).
//
// The viewer has a JavaScript copy for static exports (rankDefinitions in assets/index.html,
// between the parity markers). TestViewerParity runs both on the same cases: change them
// together.
func rankDefinitions(name string, all []Def, from string, imports map[string]resolvedImport, links []Link) []Def {
	if t, ok := imports[name]; ok {
		if t.Dir {
			var in []Def
			for _, d := range all {
				if path.Dir(d.File) == t.Path {
					in = append(in, d)
				}
			}
			if len(in) > 0 {
				return in
			}
		} else {
			return []Def{{File: t.Path, Line: t.Line, Kind: "import"}}
		}
	}
	for _, d := range all {
		if d.File == from {
			return []Def{d}
		}
	}
	near := map[string]int{}
	for _, l := range links {
		p := l.Path
		if !l.Dir {
			p = path.Dir(p)
		}
		near[p] = 2
	}
	near[path.Dir(from)] = max(near[path.Dir(from)], 1)
	out := append([]Def{}, all...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := near[path.Dir(out[i].File)], near[path.Dir(out[j].File)]
		if a != b {
			return a > b
		}
		return out[i].File < out[j].File
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
