package main

// Language packs for the code panel: find import-like references in a source file and
// resolve them to files in the repo. Each pack is a function from (file, text) to links.
// Only references that resolve are returned, so the viewer never shows a dead link.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Link is one clickable reference in a file.
type Link struct {
	Line   int    `json:"line"`             // 1-based
	Text   string `json:"text"`             // exact text to make clickable on that line
	Nth    int    `json:"nth"`              // which occurrence of Text on the line (0-based)
	Path   string `json:"path"`             // target file, or folder when Dir
	Symbol string `json:"symbol,omitempty"` // name to scroll to in the target
	Dir    bool   `json:"dir,omitempty"`    // target is a folder (Go package)
	Lang   string `json:"-"`
}

// Target of a name imported into a file (used by go-to-definition).
type importTarget struct{ Path, Symbol string }

type fileLinks struct {
	Links      []Link
	Imports    map[string]importTarget // local name -> where it comes from
	Unresolved []string                // local-looking references that didn't resolve
	Lang       string
}

// ---------- repo context ----------

type alias struct{ prefix, target string }

type repoCtx struct {
	set       map[string]bool
	files     []string
	lower     map[string]string   // lowercase path -> real path
	byBase    map[string][]string // file name -> paths
	dirs      map[string]bool     // folders that contain files
	jsAliases []alias             // "@/" -> "src/"
	jsBase    string              // tsconfig baseUrl (bare imports resolve here)
	phpPrefix []alias             // "App\" -> "src/"
	roots     []string            // extra roots for Python/PHP lookups
	goModule  string
	godotRoot string
	jsDeps    map[string]bool // package.json dependencies (nil = no package.json)
}

type ctxCache struct {
	mu  sync.Mutex
	at  time.Time
	ctx *repoCtx
}

var repoCache = &ctxCache{}

// getRepoCtx returns the repo context, rebuilt at most every few seconds (it re-reads
// the file list, tsconfig/composer/go.mod and the docs config's link rules).
func getRepoCtx(docsDir string) *repoCtx {
	repoCache.mu.Lock()
	defer repoCache.mu.Unlock()
	if repoCache.ctx != nil && time.Since(repoCache.at) < 5*time.Second {
		return repoCache.ctx
	}
	var cfg *Config
	if docsDir != "" {
		cfg = loadConfig(docsDir)
	}
	repoCache.ctx = buildRepoCtx(sourceFiles(secretGlobs), cfg)
	repoCache.at = time.Now()
	return repoCache.ctx
}

var (
	reJSONLineComment  = regexp.MustCompile(`(?m)^\s*//.*$`)
	reJSONBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reJSONTrailing     = regexp.MustCompile(`,(\s*[}\]])`)
	reGoModule         = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

// readJSONC reads a JSON-with-comments file (tsconfig, jsconfig) leniently.
func readJSONC(file string, v any) bool {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return false
	}
	s := reJSONBlockComment.ReplaceAllString(string(b), "")
	s = reJSONLineComment.ReplaceAllString(s, "")
	s = reJSONTrailing.ReplaceAllString(s, "$1")
	return jsonUnmarshal([]byte(s), v) == nil
}

func buildRepoCtx(files []string, cfg *Config) *repoCtx {
	c := &repoCtx{set: map[string]bool{}, files: files, lower: map[string]string{}, byBase: map[string][]string{}, dirs: map[string]bool{}}
	for _, f := range files {
		c.set[f] = true
		c.lower[strings.ToLower(f)] = f
		c.byBase[path.Base(f)] = append(c.byBase[path.Base(f)], f)
		for d := path.Dir(f); d != "." && !c.dirs[d]; d = path.Dir(d) {
			c.dirs[d] = true
		}
	}
	// tsconfig / jsconfig paths
	for _, name := range []string{"tsconfig.json", "jsconfig.json", "tsconfig.app.json"} {
		var tc struct {
			CompilerOptions struct {
				BaseURL string              `json:"baseUrl"`
				Paths   map[string][]string `json:"paths"`
			} `json:"compilerOptions"`
		}
		if !c.set[name] || !readJSONC(name, &tc) {
			continue
		}
		base := path.Clean(tc.CompilerOptions.BaseURL)
		if tc.CompilerOptions.BaseURL != "" && c.jsBase == "" {
			c.jsBase = base
		}
		keys := sortedKeys(tc.CompilerOptions.Paths)
		for _, k := range keys {
			for _, t := range tc.CompilerOptions.Paths[k] {
				c.jsAliases = append(c.jsAliases, alias{strings.TrimSuffix(k, "*"), path.Join(base, strings.TrimSuffix(t, "*")) + "/"})
			}
		}
	}
	// package.json dependencies: a bare import that isn't one of these looks local.
	if c.set["package.json"] {
		var pj map[string]any
		if readJSONC("package.json", &pj) {
			c.jsDeps = map[string]bool{}
			for _, k := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
				if m, ok := pj[k].(map[string]any); ok {
					for name := range m {
						c.jsDeps[name] = true
					}
				}
			}
		}
	}
	// composer.json PSR-4
	if c.set["composer.json"] {
		var cj struct {
			Autoload struct {
				Psr4 map[string]any `json:"psr-4"`
			} `json:"autoload"`
			AutoloadDev struct {
				Psr4 map[string]any `json:"psr-4"`
			} `json:"autoload-dev"`
		}
		if readJSONC("composer.json", &cj) {
			for _, m := range []map[string]any{cj.Autoload.Psr4, cj.AutoloadDev.Psr4} {
				for ns, v := range m {
					switch t := v.(type) {
					case string:
						c.phpPrefix = append(c.phpPrefix, alias{ns, t})
					case []any:
						for _, x := range t {
							if s, ok := x.(string); ok {
								c.phpPrefix = append(c.phpPrefix, alias{ns, s})
							}
						}
					}
				}
			}
		}
	}
	// go.mod
	if c.set["go.mod"] {
		if b, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
			if m := reGoModule.FindStringSubmatch(string(b)); m != nil {
				c.goModule = m[1]
			}
		}
	}
	// Godot project root (res://)
	if gp := c.byBase["project.godot"]; len(gp) > 0 {
		sort.Strings(gp)
		c.godotRoot = path.Dir(gp[0])
	}
	// Per-repo rules from docs.config.json (written by /codedocs init).
	if cfg != nil && cfg.Links != nil {
		for _, k := range sortedKeys(cfg.Links.Aliases) {
			t := cfg.Links.Aliases[k]
			if strings.Contains(k, `\`) {
				c.phpPrefix = append(c.phpPrefix, alias{k, t})
			} else {
				c.jsAliases = append([]alias{{k, t}}, c.jsAliases...)
			}
		}
		c.roots = append(c.roots, cfg.Links.Roots...)
	}
	// Longest prefix first.
	sort.SliceStable(c.jsAliases, func(i, j int) bool { return len(c.jsAliases[i].prefix) > len(c.jsAliases[j].prefix) })
	sort.SliceStable(c.phpPrefix, func(i, j int) bool { return len(c.phpPrefix[i].prefix) > len(c.phpPrefix[j].prefix) })
	return c
}

// first returns the first candidate that is a file in the repo.
func (c *repoCtx) first(cands ...string) string {
	for _, p := range cands {
		p = path.Clean(p)
		if strings.HasPrefix(p, "../") || p == ".." || strings.HasPrefix(p, "/") {
			continue
		}
		if c.set[p] {
			return p
		}
	}
	return ""
}

// ---------- shared helpers ----------

// lineCol converts a byte offset to a 1-based line and the line's start offset.
type lineIndex []int

func newLineIndex(text string) lineIndex {
	idx := lineIndex{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (li lineIndex) at(off int) (line, start int) {
	n := sort.Search(len(li), func(i int) bool { return li[i] > off }) - 1
	return n + 1, li[n]
}

type collector struct {
	text  string
	li    lineIndex
	out   *fileLinks
	seen  map[[2]int]bool
	unres map[string]bool
}

func newCollector(text, lang string) *collector {
	return &collector{text: text, li: newLineIndex(text), out: &fileLinks{Imports: map[string]importTarget{}, Lang: lang},
		seen: map[[2]int]bool{}, unres: map[string]bool{}}
}

// add links text[start:end] to target.
func (c *collector) add(start, end int, target, symbol string, dir bool) {
	if start < 0 || end <= start || target == "" || c.seen[[2]int{start, end}] {
		return
	}
	c.seen[[2]int{start, end}] = true
	line, ls := c.li.at(start)
	txt := c.text[start:end]
	nth := strings.Count(c.text[ls:start], txt)
	c.out.Links = append(c.out.Links, Link{Line: line, Text: txt, Nth: nth, Path: target, Symbol: symbol, Dir: dir})
}

func (c *collector) miss(spec string) { c.unres[spec] = true }

func (c *collector) done() *fileLinks {
	for k := range c.unres {
		c.out.Unresolved = append(c.out.Unresolved, k)
	}
	sort.Strings(c.out.Unresolved)
	sort.SliceStable(c.out.Links, func(i, j int) bool { return c.out.Links[i].Line < c.out.Links[j].Line })
	return c.out
}

// indexWord finds name as a whole word inside text[from:to]; -1 if absent.
func indexWord(text string, from, to int, name string) int {
	seg := text[from:to]
	for off := 0; ; {
		i := strings.Index(seg[off:], name)
		if i < 0 {
			return -1
		}
		i += off
		before := i == 0 || !isIdent(seg[i-1])
		after := i+len(name) >= len(seg) || !isIdent(seg[i+len(name)])
		if before && after {
			return from + i
		}
		off = i + 1
	}
}

func isIdent(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// ---------- dispatch ----------

var langByExt = map[string]string{
	"ts": "js", "tsx": "js", "js": "js", "jsx": "js", "mjs": "js", "cjs": "js", "mts": "js", "cts": "js", "vue": "js", "svelte": "js", "astro": "js",
	"php": "php", "go": "go", "py": "py",
	"gd": "godot", "tscn": "godot", "tres": "godot", "godot": "godot",
	"css": "css", "scss": "css", "sass": "css", "less": "css",
}

func linksFor(c *repoCtx, file, text string) *fileLinks {
	lang := langByExt[extOf(file)]
	col := newCollector(text, lang)
	switch lang {
	case "js":
		jsLinks(c, file, col)
		if extOf(file) == "vue" || extOf(file) == "svelte" {
			cssLinks(c, file, col)
		}
	case "php":
		phpLinks(c, file, col)
	case "go":
		goLinks(c, file, col)
	case "py":
		pyLinks(c, file, col)
	case "godot":
		godotLinks(c, file, col)
	case "css":
		cssLinks(c, file, col)
	}
	return col.done()
}

// ---------- JS / TS ----------

var (
	reJSFrom     = regexp.MustCompile(`(?s)\b(import|export)\s+(type\s+)?([\w$\s{},*]*?)\s*\bfrom\s*(['"])([^'"\n]+)['"]`)
	reJSBare     = regexp.MustCompile(`(?m)^\s*import\s*['"]([^'"\n]+)['"]`)
	reJSCall     = regexp.MustCompile(`\b(?:require|import)\s*\(\s*['"]([^'"\n]+)['"]\s*\)`)
	reJSBraces   = regexp.MustCompile(`(?s)\{(.*?)\}`)
	reJSSpecPart = regexp.MustCompile(`^(?:type\s+)?([A-Za-z_$][\w$]*)(?:\s+as\s+([A-Za-z_$][\w$]*))?$`)
	reJSDefault  = regexp.MustCompile(`^\s*([A-Za-z_$][\w$]*)\s*(?:,|$)`)
	reJSStar     = regexp.MustCompile(`\*\s+as\s+([A-Za-z_$][\w$]*)`)
)

var jsExts = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts", ".vue", ".svelte", ".json", ".css", ".scss"}

func (c *repoCtx) resolveJS(from, spec string) string {
	var bases []string
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		bases = []string{path.Join(path.Dir(from), spec)}
	default:
		for _, a := range c.jsAliases {
			if strings.HasPrefix(spec, a.prefix) || spec+"/" == a.prefix {
				bases = append(bases, path.Join(a.target, strings.TrimPrefix(spec, a.prefix)))
			}
		}
		if len(bases) == 0 && (strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/")) {
			bases = []string{path.Join("src", spec[2:]), spec[2:]}
		}
		if c.jsBase != "" && c.jsBase != "." {
			bases = append(bases, path.Join(c.jsBase, spec))
		}
		for _, r := range c.roots {
			bases = append(bases, path.Join(r, spec))
		}
	}
	for _, base := range bases {
		cands := []string{base}
		if ext := path.Ext(base); ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs" {
			stem := strings.TrimSuffix(base, ext)
			cands = append(cands, stem+".ts", stem+".tsx", stem+".mts", stem+".cts")
		}
		for _, e := range jsExts {
			cands = append(cands, base+e)
		}
		for _, e := range jsExts {
			cands = append(cands, base+"/index"+e)
		}
		if p := c.first(cands...); p != "" {
			return p
		}
	}
	return ""
}

var nodeBuiltins = set("assert", "buffer", "child_process", "crypto", "events", "fs", "http", "https", "net", "os", "path", "process",
	"stream", "string_decoder", "timers", "tls", "url", "util", "worker_threads", "zlib", "readline", "module", "dns", "perf_hooks", "electron")

// jsLocal: does an unresolved specifier look like it should point into the repo?
func (c *repoCtx) jsLocal(spec string) bool {
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/") {
		return true
	}
	if c.jsDeps == nil || strings.HasPrefix(spec, "node:") || strings.Contains(spec, ":") {
		return false
	}
	name := spec
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) > 1 {
		name = parts[0] + "/" + parts[1]
	} else {
		name = parts[0]
	}
	return !c.jsDeps[name] && !nodeBuiltins[name] && !strings.HasPrefix(name, "@types/")
}

func jsLinks(c *repoCtx, file string, col *collector) {
	t := col.text
	link := func(specStart, specEnd int) string {
		spec := t[specStart:specEnd]
		target := c.resolveJS(file, spec)
		if target != "" {
			col.add(specStart, specEnd, target, "", false)
		} else if c.jsLocal(spec) {
			col.miss(spec)
		}
		return target
	}
	for _, m := range reJSFrom.FindAllStringSubmatchIndex(t, -1) {
		target := link(m[10], m[11])
		if target == "" {
			continue
		}
		cs, ce := m[6], m[7] // the import clause
		clause := t[cs:ce]
		if b := reJSBraces.FindStringSubmatchIndex(clause); b != nil {
			inner := clause[b[2]:b[3]]
			off := cs + b[2]
			pos := 0
			for _, part := range strings.Split(inner, ",") {
				p := strings.TrimSpace(part)
				if pm := reJSSpecPart.FindStringSubmatch(p); pm != nil && pm[1] != "default" {
					if i := indexWord(t, off+pos, off+pos+len(part), pm[1]); i >= 0 {
						col.add(i, i+len(pm[1]), target, pm[1], false)
					}
					local := pm[1]
					if pm[2] != "" {
						local = pm[2]
					}
					col.out.Imports[local] = importTarget{target, pm[1]}
				}
				pos += len(part) + 1
			}
		}
		rest := reJSBraces.ReplaceAllString(clause, "")
		if t[m[2]:m[3]] == "import" {
			if dm := reJSDefault.FindStringSubmatch(rest); dm != nil && dm[1] != "type" {
				if i := indexWord(t, cs, ce, dm[1]); i >= 0 {
					col.add(i, i+len(dm[1]), target, dm[1], false)
				}
				col.out.Imports[dm[1]] = importTarget{target, ""}
			}
			if sm := reJSStar.FindStringSubmatch(rest); sm != nil {
				col.out.Imports[sm[1]] = importTarget{target, ""}
			}
		}
	}
	for _, m := range reJSBare.FindAllStringSubmatchIndex(t, -1) {
		link(m[2], m[3])
	}
	for _, m := range reJSCall.FindAllStringSubmatchIndex(t, -1) {
		link(m[2], m[3])
	}
}

// ---------- PHP ----------

var (
	rePHPNamespace = regexp.MustCompile(`(?m)^\s*namespace\s+([\w\\]+)\s*[;{]`)
	rePHPGroupUse  = regexp.MustCompile(`(?m)^\s*use\s+(?:function\s+|const\s+)?([\w\\]+)\\\{([^}]*)\}\s*;`)
	rePHPUse       = regexp.MustCompile(`(?m)^\s*use\s+(function\s+|const\s+)?([^;{]+);`)
	rePHPQualified = regexp.MustCompile(`\\?\b[A-Za-z_]\w*(?:\\[A-Za-z_]\w*)+\b`)
	rePHPAs        = regexp.MustCompile(`^\s*\\?([\w\\]+?)(?:\s+as\s+(\w+))?\s*$`)
)

// resolvePHP maps a fully qualified class name to a file: PSR-4 prefixes first, then
// the namespace path under the repo root or a known root, then by path suffix (covers
// XenForo's src/XF and src/addons/Vendor/Addon layouts, and repos that are one addon).
func (c *repoCtx) resolvePHP(fqn string) string {
	fqn = strings.TrimPrefix(fqn, `\`)
	if fqn == "" {
		return ""
	}
	rel := strings.ReplaceAll(fqn, `\`, "/") + ".php"
	for _, a := range c.phpPrefix {
		if strings.HasPrefix(fqn+`\`, a.prefix) || strings.HasPrefix(fqn, a.prefix) {
			if p := c.first(path.Join(a.target, strings.ReplaceAll(strings.TrimPrefix(fqn, a.prefix), `\`, "/")+".php")); p != "" {
				return p
			}
		}
	}
	cands := []string{rel, "src/" + rel, "src/addons/" + rel, "lib/" + rel, "app/" + rel, "upload/src/" + rel, "upload/src/addons/" + rel}
	for _, r := range c.roots {
		cands = append(cands, path.Join(r, rel))
	}
	if p := c.first(cands...); p != "" {
		return p
	}
	// Suffix match: drop leading namespace segments until a file path ends with the rest.
	// A two-segment tail (Namespace\Class) must be unambiguous; longer tails take the shortest path.
	segs := strings.Split(fqn, `\`)
	for i := 0; i < len(segs)-1; i++ {
		tail := strings.Join(segs[i:], "/") + ".php"
		var hits []string
		for _, f := range c.byBase[path.Base(tail)] {
			if f == tail || strings.HasSuffix(f, "/"+tail) {
				hits = append(hits, f)
			}
		}
		if len(hits) == 1 || (len(hits) > 1 && len(segs)-i >= 3) {
			sort.Slice(hits, func(a, b int) bool { return len(hits[a]) < len(hits[b]) })
			return hits[0]
		}
	}
	if p, ok := c.lower[strings.ToLower(rel)]; ok {
		return p
	}
	return ""
}

func phpLinks(c *repoCtx, file string, col *collector) {
	t := col.text
	nsLines := map[int]bool{}
	for _, m := range rePHPNamespace.FindAllStringIndex(t, -1) {
		l, _ := col.li.at(m[0])
		nsLines[l] = true
	}
	short := func(fqn string) string { return fqn[strings.LastIndex(fqn, `\`)+1:] }
	// Group use: use XF\Mvc\{Entity\Finder, Reply\View as V};
	for _, m := range rePHPGroupUse.FindAllStringSubmatchIndex(t, -1) {
		prefix := t[m[2]:m[3]]
		inner, off := t[m[4]:m[5]], m[4]
		pos := 0
		for _, part := range strings.Split(inner, ",") {
			if pm := rePHPAs.FindStringSubmatch(part); pm != nil {
				fqn := prefix + `\` + pm[1]
				if target := c.resolvePHP(fqn); target != "" {
					if i := strings.Index(t[off+pos:off+pos+len(part)], pm[1]); i >= 0 {
						col.add(off+pos+i, off+pos+i+len(pm[1]), target, short(fqn), false)
					}
					local := short(fqn)
					if pm[2] != "" {
						local = pm[2]
					}
					col.out.Imports[local] = importTarget{target, short(fqn)}
				}
			}
			pos += len(part) + 1
		}
	}
	// Plain use statements: record aliases (the qualified names get linked below).
	for _, m := range rePHPUse.FindAllStringSubmatch(t, -1) {
		if m[1] != "" || strings.Contains(m[2], "{") {
			continue
		}
		for _, part := range strings.Split(m[2], ",") {
			if pm := rePHPAs.FindStringSubmatch(part); pm != nil {
				if target := c.resolvePHP(pm[1]); target != "" {
					local := short(pm[1])
					if pm[2] != "" {
						local = pm[2]
					}
					col.out.Imports[local] = importTarget{target, short(pm[1])}
				}
			}
		}
	}
	// Every qualified name (use lines, extends, new \X\Y, X\Y::class, docblocks).
	for _, m := range rePHPQualified.FindAllStringIndex(t, -1) {
		if l, _ := col.li.at(m[0]); nsLines[l] {
			continue
		}
		start := m[0]
		if t[start] == '\\' {
			start++
		}
		fqn := t[start:m[1]]
		if target := c.resolvePHP(fqn); target != "" && target != file {
			col.add(start, m[1], target, short(fqn), false)
		}
	}
}

// ---------- Go ----------

var (
	reGoImportBlock  = regexp.MustCompile(`(?s)\bimport\s*\((.*?)\)`)
	reGoImportSingle = regexp.MustCompile(`(?m)^\s*import\s+(?:[\w.]+\s+)?"([^"]+)"`)
	reGoImportLine   = regexp.MustCompile(`(?m)^\s*(?:([\w.]+)\s+)?"([^"]+)"`)
)

func (c *repoCtx) resolveGo(spec string) string {
	if c.goModule == "" || (spec != c.goModule && !strings.HasPrefix(spec, c.goModule+"/")) {
		return ""
	}
	dir := strings.TrimPrefix(strings.TrimPrefix(spec, c.goModule), "/")
	if dir == "" {
		dir = "."
	}
	if dir == "." || c.dirs[dir] {
		return dir
	}
	return ""
}

func goLinks(c *repoCtx, file string, col *collector) {
	t := col.text
	one := func(aliasName string, s, e int) {
		spec := t[s:e]
		dir := c.resolveGo(spec)
		if dir == "" {
			if c.goModule != "" && strings.HasPrefix(spec, c.goModule) {
				col.miss(spec)
			}
			return
		}
		col.add(s, e, dir, "", true)
		local := aliasName
		if local == "" {
			local = path.Base(spec)
		}
		col.out.Imports[local] = importTarget{dir, ""}
	}
	for _, m := range reGoImportBlock.FindAllStringSubmatchIndex(t, -1) {
		block, off := t[m[2]:m[3]], m[2]
		for _, lm := range reGoImportLine.FindAllStringSubmatchIndex(block, -1) {
			a := ""
			if lm[2] >= 0 {
				a = block[lm[2]:lm[3]]
			}
			one(a, off+lm[4], off+lm[5])
		}
	}
	for _, m := range reGoImportSingle.FindAllStringSubmatchIndex(t, -1) {
		one("", m[2], m[3])
	}
}

// ---------- Python ----------

var (
	rePyFrom   = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+(\.*[\w.]*)[ \t]+import[ \t]+(\([^)]*\)|[^\n#]+)`)
	rePyImport = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([\w.]+(?:[ \t]+as[ \t]+\w+)?(?:[ \t]*,[ \t]*[\w.]+(?:[ \t]+as[ \t]+\w+)?)*)`)
	rePyName   = regexp.MustCompile(`^\s*([A-Za-z_]\w*)(?:\s+as\s+([A-Za-z_]\w*))?\s*$`)
)

func (c *repoCtx) resolvePyModule(from, mod string) string {
	dots := len(mod) - len(strings.TrimLeft(mod, "."))
	rest := strings.ReplaceAll(strings.TrimLeft(mod, "."), ".", "/")
	var bases []string
	if dots > 0 {
		d := path.Dir(from)
		for i := 1; i < dots; i++ {
			d = path.Dir(d)
		}
		bases = []string{path.Join(d, rest)}
	} else {
		bases = []string{rest, "src/" + rest}
		for _, r := range c.roots {
			bases = append(bases, path.Join(r, rest))
		}
		bases = append(bases, path.Join(path.Dir(from), rest)) // implicit relative (scripts)
	}
	for _, b := range bases {
		if p := c.first(b+".py", b+"/__init__.py"); p != "" {
			return p
		}
	}
	return ""
}

func pyLinks(c *repoCtx, file string, col *collector) {
	t := col.text
	for _, m := range rePyFrom.FindAllStringSubmatchIndex(t, -1) {
		mod := t[m[2]:m[3]]
		modFile := c.resolvePyModule(file, mod)
		if modFile != "" && mod != "." && strings.Trim(mod, ".") != "" {
			col.add(m[2], m[3], modFile, "", false)
		} else if modFile == "" && strings.HasPrefix(mod, ".") {
			col.miss(mod)
		}
		names, off := t[m[4]:m[5]], m[4]
		if strings.HasPrefix(names, "(") {
			names, off = names[1:len(names)-1], off+1
		}
		pos := 0
		for _, part := range strings.Split(names, ",") {
			if pm := rePyName.FindStringSubmatch(strings.ReplaceAll(part, "\n", " ")); pm != nil {
				sep := "."
				if strings.HasSuffix(mod, ".") {
					sep = ""
				}
				target, sym := c.resolvePyModule(file, mod+sep+pm[1]), "" // a submodule?
				if target == "" && modFile != "" {
					target, sym = modFile, pm[1]
				}
				if target != "" {
					if i := indexWord(t, off+pos, off+pos+len(part), pm[1]); i >= 0 {
						col.add(i, i+len(pm[1]), target, sym, false)
					}
					local := pm[1]
					if pm[2] != "" {
						local = pm[2]
					}
					col.out.Imports[local] = importTarget{target, sym}
				}
			}
			pos += len(part) + 1
		}
	}
	for _, m := range rePyImport.FindAllStringSubmatchIndex(t, -1) {
		list, off := t[m[2]:m[3]], m[2]
		pos := 0
		for _, part := range strings.Split(list, ",") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				if target := c.resolvePyModule(file, fields[0]); target != "" {
					if i := strings.Index(t[off+pos:off+pos+len(part)], fields[0]); i >= 0 {
						col.add(off+pos+i, off+pos+i+len(fields[0]), target, "", false)
					}
					local := fields[0]
					if len(fields) == 3 {
						local = fields[2]
					}
					col.out.Imports[local] = importTarget{target, ""}
				}
			}
			pos += len(part) + 1
		}
	}
}

// ---------- Godot (GDScript, scenes, resources) ----------

var reResPath = regexp.MustCompile(`res://[^"'\s)]+`)

func godotLinks(c *repoCtx, file string, col *collector) {
	for _, m := range reResPath.FindAllStringIndex(col.text, -1) {
		rel := strings.TrimPrefix(col.text[m[0]:m[1]], "res://")
		if p := c.first(path.Join(c.godotRoot, rel)); p != "" {
			col.add(m[0], m[1], p, "", false)
		} else if strings.HasSuffix(rel, ".gd") || strings.HasSuffix(rel, ".tscn") {
			col.miss("res://" + rel)
		}
	}
}

// ---------- CSS / SCSS ----------

var reCSSImport = regexp.MustCompile(`@(?:import|use|forward)\s+(?:url\()?['"]([^'"]+)['"]`)

func cssLinks(c *repoCtx, file string, col *collector) {
	for _, m := range reCSSImport.FindAllStringSubmatchIndex(col.text, -1) {
		spec := col.text[m[2]:m[3]]
		if strings.Contains(spec, "://") {
			continue
		}
		base := path.Join(path.Dir(file), spec)
		if strings.HasPrefix(spec, "~") || strings.HasPrefix(spec, "@/") {
			base = path.Join("src", strings.TrimLeft(strings.TrimPrefix(spec, "@/"), "~/"))
		}
		dir, name := path.Dir(base), path.Base(base)
		var cands []string
		for _, b := range []string{base, path.Join(dir, "_"+name)} {
			cands = append(cands, b, b+".scss", b+".sass", b+".css", b+".less")
		}
		cands = append(cands, base+"/_index.scss", base+"/index.scss")
		if p := c.first(cands...); p != "" {
			col.add(m[2], m[3], p, "", false)
		} else {
			col.miss(spec)
		}
	}
}

// ---------- `codedocs links` ----------

func cmdLinks(o options) {
	docsDir := findDocsDir(o, false)
	c := getRepoCtx(docsDir)
	if len(o.pos) > 0 {
		f := strings.TrimPrefix(posix(o.pos[0]), "./")
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			die("can't read " + f)
		}
		fl := linksFor(c, f, string(b))
		if o.json {
			printJSON(map[string]any{"file": f, "lang": fl.Lang, "links": fl.Links, "unresolved": fl.Unresolved})
			return
		}
		if fl.Lang == "" {
			fmt.Printf("%s: no language pack for .%s files (Ctrl+click go-to-definition still works)\n", f, extOf(f))
			return
		}
		fmt.Printf("%s (%s): %d link(s)\n", f, fl.Lang, len(fl.Links))
		for _, l := range fl.Links {
			t := l.Path
			if l.Symbol != "" {
				t += "#" + l.Symbol
			}
			if l.Dir {
				t += "/ (package)"
			}
			fmt.Printf("  %4d  %-40s -> %s\n", l.Line, l.Text, t)
		}
		if len(fl.Unresolved) > 0 {
			fmt.Printf("Unresolved local references: %s\n", strings.Join(fl.Unresolved, ", "))
		}
		return
	}
	type stat struct {
		files, links int
		unres        map[string]string // spec -> example file
	}
	stats := map[string]*stat{}
	docsPrefix := "\x00"
	if docsDir != "" {
		docsPrefix = rel(docsDir) + "/"
	}
	for _, f := range c.files {
		lang := langByExt[extOf(f)]
		if lang == "" || strings.HasPrefix(f, docsPrefix) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil || len(b) > 1<<20 {
			continue
		}
		fl := linksFor(c, f, string(b))
		st := stats[lang]
		if st == nil {
			st = &stat{unres: map[string]string{}}
			stats[lang] = st
		}
		st.files++
		st.links += len(fl.Links)
		for _, u := range fl.Unresolved {
			if _, ok := st.unres[u]; !ok {
				st.unres[u] = f
			}
		}
	}
	if o.json {
		out := map[string]any{}
		for k, st := range stats {
			out[k] = map[string]any{"files": st.files, "links": st.links, "unresolved": st.unres}
		}
		printJSON(map[string]any{"languages": out, "detected": c.describe()})
		return
	}
	fmt.Println("Code links by language:")
	for _, k := range sortedKeys(stats) {
		st := stats[k]
		fmt.Printf("- %s: %d files, %d links, %d unresolved local reference(s)\n", k, st.files, st.links, len(st.unres))
		for i, u := range sortedKeys(st.unres) {
			if i == 12 {
				fmt.Printf("    … and %d more\n", len(st.unres)-12)
				break
			}
			fmt.Printf("    %s  (e.g. in %s)\n", u, st.unres[u])
		}
	}
	if len(stats) == 0 {
		fmt.Println("- no files with a language pack (JS/TS, PHP, Go, Python, Godot, CSS)")
	}
	fmt.Println("\nDetected:")
	for _, d := range c.describe() {
		fmt.Println("- " + d)
	}
}

func (c *repoCtx) describe() []string {
	var d []string
	for _, a := range c.jsAliases {
		d = append(d, fmt.Sprintf("JS alias %s -> %s", a.prefix, a.target))
	}
	if c.jsBase != "" {
		d = append(d, "JS baseUrl "+c.jsBase)
	}
	for _, a := range c.phpPrefix {
		d = append(d, fmt.Sprintf("PHP namespace %s -> %s", a.prefix, a.target))
	}
	if c.goModule != "" {
		d = append(d, "Go module "+c.goModule)
	}
	if c.godotRoot != "" {
		d = append(d, "Godot project at "+c.godotRoot+"/ (res://)")
	}
	for _, r := range c.roots {
		d = append(d, "extra root "+r)
	}
	if len(d) == 0 {
		d = append(d, "nothing special (relative imports only)")
	}
	return d
}
