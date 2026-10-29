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

// findDefinition: imports of the current file first, then a definition in the same file,
// then every definition ranked by closeness (folders the file imports from, same folder).
func findDefinition(c *repoCtx, from, name string) []Def {
	var fl *fileLinks
	if from != "" && c.set[from] {
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from))); err == nil {
			fl = linksFor(c, from, string(b))
			if t, ok := fl.Imports[name]; ok {
				if c.dirs[t.Path] || t.Path == "." { // Go package: look inside it
					var in []Def
					for _, d := range defs.get(c.files)[name] {
						if path.Dir(d.File) == t.Path {
							in = append(in, d)
						}
					}
					if len(in) > 0 {
						return in
					}
				} else {
					sym := t.Symbol
					if sym == "" {
						sym = name
					}
					return []Def{{File: t.Path, Line: max(1, resolveSymbol(t.Path, sym)), Kind: "import"}}
				}
			}
		}
	}
	all := defs.get(c.files)[name]
	var local []Def
	for _, d := range all {
		if d.File == from {
			local = append(local, d)
		}
	}
	if len(local) > 0 {
		return local[:1]
	}
	near := map[string]int{}
	if fl != nil {
		for _, l := range fl.Links {
			p := l.Path
			if !l.Dir {
				p = path.Dir(p)
			}
			near[p] = 2
		}
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
