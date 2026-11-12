package main

// Approximate callers: where a symbol seems to be called from. "Approximate" because it's a
// name match, not a type-checked reference: the files that can see the symbol (its own file,
// files that import it, the rest of its Go package) are searched for `name(`. The viewer
// labels the count with ≈ for that reason, the same way pages mark inferred claims.

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxCallers = 200

// importIndex maps a file to the files that import it (or, for Go, import its folder).
type importIndex struct {
	mu sync.Mutex
	at time.Time
	by map[string][]string
}

var importers = &importIndex{}

func (x *importIndex) get(c *repoCtx) map[string][]string {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.by != nil && time.Since(x.at) < 20*time.Second {
		return x.by
	}
	by := map[string][]string{}
	for _, f := range c.files {
		if langByExt[extOf(f)] == "" {
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
		seen := map[string]bool{}
		for _, l := range linksFor(c, f, string(b)).Links {
			if l.Path != f && !seen[l.Path] {
				seen[l.Path] = true
				by[l.Path] = append(by[l.Path], f)
			}
		}
	}
	x.by, x.at = by, time.Now()
	return by
}

var (
	callCache   sync.Map
	reCallNoise = regexp.MustCompile(`^\s*(?://|#|\*|/\*|import\b|from\b|use\b|require\b)`)
)

func callRegexp(name string) *regexp.Regexp {
	if v, ok := callCache.Load(name); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(`(?:^|[^\w$])` + regexp.QuoteMeta(name) + `\s*(?:<[^<>()]*>)?\(`)
	callCache.Store(name, re)
	return re
}

// findCallers lists lines that look like calls to symbol (defined in file), outside its own
// definition. Results are capped at maxCallers.
func findCallers(c *repoCtx, file, symbol string) []Def {
	name := symbol
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || !c.set[file] {
		return []Def{}
	}
	cands := []string{file}
	by := importers.get(c)
	cands = append(cands, by[file]...)
	if extOf(file) == "go" {
		dir := path.Dir(file)
		cands = append(cands, by[dir]...)
		for _, f := range c.files {
			if f != file && path.Dir(f) == dir && extOf(f) == "go" {
				cands = append(cands, f)
			}
		}
	}
	from, to, _ := symbolSpan(file, symbol)
	re := callRegexp(name)
	pats := defPatterns(extOf(file))
	out := []Def{}
	for _, f := range uniq(cands) {
		ls := fileLines(f)
		for i, ln := range ls {
			if f == file && i+1 >= from && i+1 <= to {
				continue // the definition itself (a recursive call is still a call, but rarely the point)
			}
			if len(ln) > 400 || !re.MatchString(ln) || reCallNoise.MatchString(ln) || isDefLine(ln, name, pats) {
				continue
			}
			out = append(out, Def{File: f, Line: i + 1})
			if len(out) >= maxCallers {
				return out
			}
		}
	}
	return out
}

// isDefLine: the line declares name (another method of the same name, an overload) rather than calling it.
func isDefLine(ln, name string, pats []*regexp.Regexp) bool {
	for _, re := range pats {
		if m := re.FindStringSubmatch(ln); m != nil && m[1] == name {
			return true
		}
	}
	return false
}
