package main

// Building the flow graph from the docs.
//
// The judgement calls live in the flow pages, as a ```flowgraph block that the agent writes
// (see skills/codedocs/reference/flows.md): the ordered steps of the flow, each naming a unit
// and a short label, with side effects and the branches not taken. Everything mechanical is
// done here: listing the repo's units (exported functions, class methods, route handlers),
// putting each on a layer (from .codedocs/layers.yml, then the owning component's `layer`,
// then folder and file-name heuristics) and in a district (its component, or its folder),
// resolving every step to a unit, and checking the result against the schema's rules.
//
// A block looks like this:
//
//	```flowgraph
//	color: #a78bfa
//	`src/ui/auth/SignupForm.tsx#SignupForm` | Form submitted
//	branch | inline errors
//	route POST /signup | POST /signup
//	`src/services/users.ts#UserService.create` | Create user
//	store users | Insert user | db-write
//	external Mail provider | Send welcome email
//	```

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- flowgraph blocks in flow pages ----------

type unitRef struct {
	Kind     string // code, store, file, queue, cache, external, route
	File     string
	Symbol   string
	Line     int
	End      int
	Name     string // stores, externals: the name; routes: "METHOD /path"
	District string // stores: optional "postgres" in `store postgres/users`
	Raw      string
}

type blockStep struct {
	Ref    unitRef
	Label  string
	Effect string
	Line   int // line in the page body
}

type blockBranch struct {
	At     int
	Label  string
	Target string
}

type blockErr struct {
	Line int
	Msg  string
}

type flowBlock struct {
	Page                          *Page
	Line                          int
	ID, Name, Color, Lens, Parent string
	Steps                         []blockStep
	Branches                      []blockBranch
	Errs                          []blockErr
}

var (
	reFlowFence  = regexp.MustCompile("^\\s*(```|~~~)\\s*flowgraph\\s*$")
	reBlockKey   = regexp.MustCompile(`^(id|name|color|lens|parent)\s*:\s*(.*)$`)
	reBareRef    = regexp.MustCompile(`^([A-Za-z0-9_.\-/@~+]+\.[A-Za-z0-9]+)(?::(\d+)(?:-(\d+))?)?(?:#([A-Za-z_$][\w$.]*))?$`)
	reRouteRef   = regexp.MustCompile(`^(?i:(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|ANY|ALL)\s+)?(/\S*)$`)
	unitKeywords = set("store", "file", "queue", "cache", "external", "route")
)

// isFlowFence: stripCode keeps flowgraph blocks, so their code refs are checked like any other.
func isFlowFence(line string) bool { return reFlowFence.MatchString(line) }

func flowBlocks(p *Page) []*flowBlock {
	var out []*flowBlock
	ls := strings.Split(p.Body, "\n")
	for i := 0; i < len(ls); i++ {
		if !isFlowFence(ls[i]) {
			continue
		}
		fence := strings.TrimSpace(ls[i])[:3]
		b := &flowBlock{Page: p, Line: i + 1, ID: p.slug(), Name: p.Title, Lens: "system"}
		closed := false
		for i++; i < len(ls); i++ {
			raw := strings.TrimRight(ls[i], "\r")
			if strings.HasPrefix(strings.TrimSpace(raw), fence) {
				closed = true
				break
			}
			b.parseLine(strings.TrimSpace(raw), i+1)
		}
		if !closed {
			b.Errs = append(b.Errs, blockErr{b.Line, "the flowgraph block is never closed"})
		}
		if len(b.Steps) == 0 && len(b.Errs) == 0 {
			b.Errs = append(b.Errs, blockErr{b.Line, "the flowgraph block has no steps"})
		}
		out = append(out, b)
	}
	return out
}

func (b *flowBlock) parseLine(l string, line int) {
	if l == "" || strings.HasPrefix(l, "//") {
		return
	}
	bad := func(f string, a ...any) { b.Errs = append(b.Errs, blockErr{line, fmt.Sprintf(f, a...)}) }
	if !strings.Contains(l, "|") {
		if m := reBlockKey.FindStringSubmatch(l); m != nil {
			v := strings.Trim(strings.TrimSpace(m[2]), `"'`)
			switch m[1] {
			case "id":
				b.ID = v
			case "name":
				b.Name = v
			case "color":
				if !reHexColor.MatchString(v) {
					bad("color %q: use #rrggbb", v)
				}
				b.Color = v
			case "lens":
				b.Lens = v
			case "parent":
				b.Parent = v
			}
			return
		}
		bad("a step is `unit | label` or `unit | label | effect`; this line has no |: %s", l)
		return
	}
	parts := strings.Split(l, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if strings.EqualFold(parts[0], "branch") {
		if len(b.Steps) == 0 {
			bad("a branch must follow the step it leaves from")
			return
		}
		if len(parts) < 2 || parts[1] == "" {
			bad("a branch needs a label: branch | 409 email taken")
			return
		}
		br := blockBranch{At: len(b.Steps) - 1, Label: parts[1]}
		if len(parts) > 2 {
			br.Target = strings.TrimSpace(strings.TrimLeft(parts[2], "->→ "))
		}
		b.Branches = append(b.Branches, br)
		return
	}
	if len(parts) < 2 || parts[1] == "" {
		bad("this step has no label: %s", l)
		return
	}
	ref, err := parseUnitRef(parts[0])
	if err != "" {
		bad("%s", err)
		return
	}
	st := blockStep{Ref: ref, Label: parts[1], Line: line}
	if len(parts) > 2 && parts[2] != "" {
		st.Effect = strings.ToLower(parts[2])
		if !stepEffects[st.Effect] {
			bad("effect %q: use db-read, db-write, external, disk-write, queue or cache", parts[2])
			return
		}
	} else if ref.Kind == "external" {
		st.Effect = "external"
	}
	b.Steps = append(b.Steps, st)
}

func parseUnitRef(s string) (unitRef, string) {
	s = strings.TrimSpace(s)
	r := unitRef{Raw: s}
	if kw, rest, ok := strings.Cut(s, " "); ok && unitKeywords[strings.ToLower(kw)] {
		r.Kind, r.Name = strings.ToLower(kw), strings.TrimSpace(rest)
		if r.Name == "" {
			return r, kw + " needs a name"
		}
		switch r.Kind {
		case "route":
			m := reRouteRef.FindStringSubmatch(r.Name)
			if m == nil {
				return r, "a route is `route METHOD /path`, e.g. route POST /signup"
			}
			r.Name = routeKey(m[1], m[2])
		case "store", "file", "queue", "cache":
			if d, n, ok := strings.Cut(r.Name, "/"); ok && d != "" && n != "" {
				r.District, r.Name = strings.TrimSpace(d), strings.TrimSpace(n)
			}
		}
		return r, ""
	}
	m := reBareRef.FindStringSubmatch(strings.Trim(s, "`"))
	if m == nil {
		return r, fmt.Sprintf("can't read %q as a unit: use a code ref (`path#Symbol`), or store, file, queue, cache, external or route followed by a name", s)
	}
	r.Kind, r.File, r.Symbol = "code", strings.TrimPrefix(m[1], "./"), m[4]
	r.Line, _ = strconv.Atoi(m[2])
	r.End, _ = strconv.Atoi(m[3])
	return r, ""
}

func routeKey(method, p string) string {
	method = strings.ToUpper(method)
	if method == "" || method == "ALL" {
		method = "ANY"
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/"
	}
	return method + " " + p
}

// ---------- layers ----------

type layerRules struct {
	File   string
	Order  []string
	Globs  map[string][]string
	Ignore []string
	Errs   []string
}

var defaultLayers = []GLayer{{"ui", "UI", 0}, {"api", "API", 1}, {"svc", "Services", 2}, {"data", "Data", 3}}

// Folder and file names that say which tier code belongs to. The deepest match wins.
var tierWords = func() map[string]string {
	m := map[string]string{}
	add := func(layer string, words ...string) {
		for _, w := range words {
			m[w] = layer
		}
	}
	add("ui", "components", "component", "pages", "views", "view", "ui", "screens", "screen", "widgets", "layouts", "layout",
		"frontend", "client", "templates", "hooks", "styles", "islands", "partials")
	add("api", "api", "apis", "routes", "route", "router", "routers", "controllers", "controller", "handlers", "handler",
		"endpoints", "middleware", "middlewares", "http", "graphql", "resolvers", "rest", "webhooks", "rpc", "grpc", "server")
	add("svc", "services", "service", "domain", "core", "usecases", "use-cases", "application", "jobs", "workers", "tasks",
		"business", "logic", "managers", "providers", "lib", "libs", "commands", "cli")
	add("data", "models", "model", "db", "database", "migrations", "repositories", "repository", "repos", "schema", "schemas",
		"entities", "entity", "dao", "persistence", "prisma", "sql", "seeds", "seeders", "orm", "data", "datastore", "stores")
	return m
}()

var reStemTier = []struct {
	re    *regexp.Regexp
	layer string
}{
	{regexp.MustCompile(`(?i)[._-](component|page|view|screen|layout|widget)$`), "ui"},
	{regexp.MustCompile(`(?i)([._-](controller|routes?|router|handler|middleware|resolver)|^(routes?|router|handlers?|middleware))$`), "api"},
	{regexp.MustCompile(`(?i)[._-]?(service|usecase|job|worker)$`), "svc"},
	{regexp.MustCompile(`(?i)[._-](model|entity|repository|repo|schema|migration|dao)$`), "data"},
}

var uiExt = set("tsx", "jsx", "vue", "svelte", "html", "htm", "css", "scss", "twig", "blade", "cshtml", "razor")

func heuristicLayer(f string) string {
	base := path.Base(f)
	stem := strings.TrimSuffix(base, path.Ext(base))
	stem = strings.TrimSuffix(stem, ".blade")
	for _, s := range reStemTier {
		if s.re.MatchString(stem) {
			return s.layer
		}
	}
	segs := strings.Split(strings.ToLower(path.Dir(f)), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if l, ok := tierWords[segs[i]]; ok {
			return l
		}
	}
	if uiExt[extOf(f)] {
		return "ui"
	}
	if extOf(f) == "sql" {
		return "data"
	}
	return ""
}

func layersFile() string { return filepath.Join(root, ".codedocs", "layers.yml") }

// loadLayerRules reads .codedocs/layers.yml: each top-level key is a layer id (top to
// bottom), holding a list of globs. `ignore` lists globs to leave out of the graph.
//
//	ui:
//	  - src/components/**
//	api: [src/routes/**, src/middleware/**]
//	ignore:
//	  - scripts/**
func loadLayerRules() layerRules {
	r := layerRules{File: layersFile(), Globs: map[string][]string{}}
	b, err := os.ReadFile(r.File)
	if err != nil {
		return r
	}
	cur := ""
	for n, raw := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		l := raw
		if i := strings.Index(l, " #"); i >= 0 {
			l = l[:i]
		}
		if strings.HasPrefix(strings.TrimSpace(l), "#") || strings.TrimSpace(l) == "" {
			continue
		}
		item := func(v string) {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if v == "" {
				return
			}
			if cur == "ignore" {
				r.Ignore = append(r.Ignore, v)
			} else {
				r.Globs[cur] = append(r.Globs[cur], v)
			}
		}
		if l[0] != ' ' && l[0] != '\t' && l[0] != '-' {
			k, v, ok := strings.Cut(l, ":")
			k = strings.TrimSpace(k)
			if !ok || !reGraphID.MatchString(k) || strings.ContainsAny(k, "/.:$") {
				r.Errs = append(r.Errs, fmt.Sprintf("%s:%d: expected `layer-id:`", rel(r.File), n+1))
				cur = ""
				continue
			}
			cur = k
			if cur != "ignore" && r.Globs[cur] == nil {
				r.Order = append(r.Order, cur)
				r.Globs[cur] = []string{}
			}
			if v = strings.TrimSpace(v); strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
				for _, g := range strings.Split(v[1:len(v)-1], ",") {
					item(g)
				}
			} else if v != "" {
				item(v)
			}
			continue
		}
		if cur == "" {
			r.Errs = append(r.Errs, fmt.Sprintf("%s:%d: a glob outside any layer", rel(r.File), n+1))
			continue
		}
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "-") {
			r.Errs = append(r.Errs, fmt.Sprintf("%s:%d: expected `- glob`", rel(r.File), n+1))
			continue
		}
		item(strings.TrimPrefix(t, "-"))
	}
	return r
}

func layerName(id string) string {
	for _, l := range defaultLayers {
		if l.ID == id {
			return l.Name
		}
	}
	s := strings.NewReplacer("-", " ", "_", " ").Replace(id)
	if s == "" {
		return id
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---------- units in the code ----------

type codeUnit struct {
	File   string
	Symbol string // qualified: Class.method; empty for a whole-file unit
	Name   string // what the view shows: the symbol, or "POST /signup" for a route
	Route  string // routeKey, for route handlers
	Line   int
	End    int
}

var reTestFile = regexp.MustCompile(`(^|/)(__tests__|__mocks__|tests?|specs?|e2e|testdata|fixtures?|mocks?)/|[._-](test|spec|stories)\.\w+$|_test\.go$|(^|/)(test_[^/]+|conftest)\.py$|\.d\.ts$|(^|/)[^/]+\.config\.\w+$`)

var (
	reJSFn      = regexp.MustCompile(`^export\s+(?:default\s+)?(?:declare\s+)?(?:async\s+)?function\*?\s+([A-Za-z_$][\w$]*)`)
	reJSClass   = regexp.MustCompile(`^export\s+(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)`)
	reJSConst   = regexp.MustCompile(`^export\s+(?:const|let)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(.*)$`)
	reJSCJS     = regexp.MustCompile(`^(?:module\.)?exports\.([A-Za-z_$][\w$]*)\s*=`)
	reJSFnValue = regexp.MustCompile(`^(?:async\b|function\b|\(|[A-Za-z_$][\w$]*\s*=>|(?:React\.)?(?:memo|forwardRef)\b|styled\b)`)
	reJSMethod  = regexp.MustCompile(`^\s+(?:(?:public|protected|static|async|override|readonly|get|set)\s+)*([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(`)
	reJSRoute   = regexp.MustCompile("\\b(?:app|router|server|api|fastify|routes|r)\\.(get|post|put|patch|delete|all|options|head)\\(\\s*['\"`]([^'\"`]+)['\"`]")
	reGoFunc    = regexp.MustCompile(`^func\s+(?:\(\s*\w*\s*\*?\s*([A-Za-z_]\w*)(?:\[[^\]]*\])?\s*\)\s*)?([A-Za-z_]\w*)\s*[\[(]`)
	reGoRoute   = regexp.MustCompile(`\.(HandleFunc|Handle|Get|Post|Put|Patch|Delete|GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]+)"`)
	rePyDef     = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z]\w*)`)
	rePyClass   = regexp.MustCompile(`^class\s+([A-Za-z]\w*)`)
	rePyMethod  = regexp.MustCompile(`^(\s+)(?:async\s+)?def\s+([A-Za-z]\w*)`)
	rePyRoute   = regexp.MustCompile(`^@\w+(?:\.\w+)*\.(get|post|put|patch|delete|route|api_route)\(\s*['"]([^'"]+)['"]`)
	rePHPClass  = regexp.MustCompile(`^\s*(?:(?:abstract|final|readonly)\s+)*class\s+(\w+)`)
	rePHPMethod = regexp.MustCompile(`^\s+(?:public\s+)?(?:static\s+)?function\s+(\w+)`)
	rePHPFn     = regexp.MustCompile(`^function\s+(\w+)`)
	rePHPRoute  = regexp.MustCompile(`(?:Route::|\$(?:app|router|r)->)(get|post|put|patch|delete|any|match)\(\s*['"]([^'"]+)['"]`)
	reGenFn     = regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:export\s+)?(?:async\s+)?(?:fn|func|fun|def|function)\s+([A-Za-z_]\w*)`)
	reGenClass  = regexp.MustCompile(`^(?:public\s+|export\s+|pub\s+|internal\s+)?(?:(?:abstract|final|sealed|data|open|static|partial)\s+)*(?:class|struct|object|module|impl(?:<[^>]*>)?(?:\s+[\w:<>]+\s+for)?)\s+([A-Za-z_]\w*)`)
	reGenMethod = regexp.MustCompile(`^\s+(?:pub\s+(?:async\s+)?fn\s+([A-Za-z_]\w*)|def\s+([A-Za-z_]\w*)|public\s+(?:(?:static|async|override|virtual|final|synchronized)\s+)*[\w<>\[\],.?]+(?:\s+[\w<>\[\],.?]+)*?\s+([A-Za-z_]\w*)\s*\()`)
	jsLangExt   = set("ts", "tsx", "js", "jsx", "mjs", "cjs", "mts", "cts")
	notMethod   = set("if", "for", "while", "switch", "catch", "return", "function", "constructor", "super", "new", "typeof", "await", "else", "do", "try")
)

func indentOf2(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

// containerEnd: the last line of a top-level class or impl starting at line i (0-based):
// up to the next line at column 0 (a closing brace or `end` belongs to it).
func containerEnd(ls []string, i int) int {
	for j := i + 1; j < len(ls); j++ {
		t := strings.TrimRight(ls[j], "\r")
		if strings.TrimSpace(t) == "" || indentOf2(t) > 0 {
			continue
		}
		if strings.HasPrefix(t, "}") || strings.HasPrefix(t, "end") || strings.HasPrefix(t, ")") {
			return j
		}
		return j - 1
	}
	return len(ls) - 1
}

// extractUnits finds a file's units: exported functions and classes' public methods, and
// route handlers. Private helpers are left out; they fold into the unit that calls them.
func extractUnits(f string, ls []string) []codeUnit {
	ext := extOf(f)
	var out []codeUnit
	add := func(sym, name, route string, i int) {
		if name == "" {
			name = sym
		}
		out = append(out, codeUnit{File: f, Symbol: sym, Name: name, Route: route, Line: i + 1, End: max(i+1, blockEnd(ls, i+1, ext))})
	}
	methods := func(class string, i int, re *regexp.Regexp, pick func(m []string) string) {
		end := containerEnd(ls, i)
		body := -1
		for j := i + 1; j <= end && j < len(ls); j++ {
			t := strings.TrimRight(ls[j], "\r")
			if strings.TrimSpace(t) == "" {
				continue
			}
			if body < 0 {
				body = indentOf2(t)
			}
			if indentOf2(t) != body || strings.Contains(t, "private ") || strings.Contains(t, "protected ") {
				continue
			}
			if m := re.FindStringSubmatch(t); m != nil {
				if n := pick(m); n != "" && !notMethod[n] && !strings.HasPrefix(n, "_") && !strings.HasPrefix(n, "#") {
					add(class+"."+n, "", "", j)
				}
			}
		}
	}
	first := func(m []string) string {
		for _, s := range m[1:] {
			if s != "" && strings.TrimSpace(s) == s {
				return s
			}
		}
		return ""
	}
	switch {
	case ext == "vue" || ext == "svelte":
		name := strings.TrimSuffix(path.Base(f), "."+ext)
		out = append(out, codeUnit{File: f, Name: name, Line: 1, End: len(ls)})
	case jsLangExt[ext]:
		for i, l := range ls {
			l = strings.TrimRight(l, "\r")
			if m := reJSFn.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
			} else if m := reJSClass.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
				methods(m[1], i, reJSMethod, func(m []string) string { return m[1] })
			} else if m := reJSConst.FindStringSubmatch(l); m != nil && (reJSFnValue.MatchString(m[2]) || strings.Contains(m[2], "=>")) {
				add(m[1], "", "", i)
			} else if m := reJSCJS.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
			}
			if m := reJSRoute.FindStringSubmatch(l); m != nil {
				k := routeKey(m[1], m[2])
				add("", k, k, i)
			}
		}
	case ext == "go":
		isMain := false
		for _, l := range ls {
			if strings.HasPrefix(l, "package ") {
				isMain = strings.TrimSpace(strings.TrimPrefix(l, "package ")) == "main"
				break
			}
		}
		for i, l := range ls {
			if m := reGoFunc.FindStringSubmatch(l); m != nil {
				name := m[2]
				if name == "init" || !(isMain || isExported(name)) || (m[1] != "" && !isMain && !isExported(m[1])) {
					continue
				}
				if m[1] != "" {
					name = m[1] + "." + name
				}
				add(name, "", "", i)
			}
			if m := reGoRoute.FindStringSubmatch(l); m != nil {
				method, p := m[1], m[2]
				if strings.HasPrefix(method, "Handle") {
					method = ""
					if mm, pp, ok := strings.Cut(p, " "); ok && strings.HasPrefix(pp, "/") {
						method, p = mm, pp
					}
				}
				if strings.HasPrefix(p, "/") {
					k := routeKey(method, p)
					add("", k, k, i)
				}
			}
		}
	case ext == "py":
		route := ""
		for i, l := range ls {
			l = strings.TrimRight(l, "\r")
			if m := rePyRoute.FindStringSubmatch(l); m != nil {
				method := m[1]
				if method == "route" || method == "api_route" {
					method = ""
				}
				route = routeKey(method, m[2])
				continue
			}
			if m := rePyDef.FindStringSubmatch(l); m != nil {
				if route != "" {
					add(m[1], route, route, i)
				} else {
					add(m[1], "", "", i)
				}
				route = ""
			} else if m := rePyClass.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
				methods(m[1], i, rePyMethod, func(m []string) string { return m[2] })
				route = ""
			} else if !strings.HasPrefix(l, "@") && strings.TrimSpace(l) != "" && indentOf2(l) == 0 {
				route = ""
			}
		}
	case ext == "php":
		for i, l := range ls {
			l = strings.TrimRight(l, "\r")
			if m := rePHPClass.FindStringSubmatch(l); m != nil && indentOf2(l) <= 4 {
				add(m[1], "", "", i)
				methods(m[1], i, rePHPMethod, func(m []string) string {
					if strings.HasPrefix(m[1], "__") {
						return ""
					}
					return m[1]
				})
			} else if m := rePHPFn.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
			}
			if m := rePHPRoute.FindStringSubmatch(l); m != nil {
				k := routeKey(m[1], "/"+strings.TrimPrefix(m[2], "/"))
				add("", k, k, i)
			}
		}
	case codeExt[ext] && ext != "sql" && ext != "sh" && ext != "ps1" && ext != "bat" && ext != "gdshader" && ext != "shader":
		for i, l := range ls {
			l = strings.TrimRight(l, "\r")
			if indentOf2(l) > 0 {
				continue
			}
			if m := reGenClass.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
				methods(m[1], i, reGenMethod, first)
			} else if m := reGenFn.FindStringSubmatch(l); m != nil {
				add(m[1], "", "", i)
			}
		}
	}
	return out
}

func isExported(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }

// ---------- building the graph ----------

type graphOptions struct {
	Granularity string // "", "symbol" or "file"; "" picks file above maxUnits
	Web         string // repo.web, for exports
	Root        string // repo.root
}

const maxUnits = 2000

type graphReport struct {
	Blocks       []*flowBlock
	Problems     []Issue  // block errors and unresolved references, by page and line
	Invalid      []string // validateGraph's findings
	Unclassified []string // files no rule or heuristic placed on a layer (put on svc)
	Granularity  string
	Extracted    int
	LayerErrs    []string
	FlowPages    int // flow pages, with or without a block
}

func (r *graphReport) ok() bool {
	return len(r.Problems) == 0 && len(r.Invalid) == 0 && len(r.LayerErrs) == 0
}

var reIDBad = regexp.MustCompile(`[^A-Za-z0-9_.:/$-]+`)

func graphID(prefix, s string) string {
	id := strings.Trim(reIDBad.ReplaceAllString(s, "-"), "-./:$_")
	if id == "" {
		id = "x"
	}
	return prefix + id
}

func slugName(s string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// flowPages returns the flow pages in the order the viewer lists them.
func flowPages(pages []*Page) []*Page {
	var out []*Page
	for _, p := range pages {
		if p.Kind == "flow" {
			out = append(out, p)
		}
	}
	order := func(p *Page) float64 {
		if v, ok := p.Data["order"].(float64); ok {
			return v
		}
		return 999
	}
	sort.SliceStable(out, func(i, j int) bool {
		if order(out[i]) != order(out[j]) {
			return order(out[i]) < order(out[j])
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

func hasFlowBlocks(pages []*Page) bool {
	for _, p := range pages {
		if p.Kind == "flow" && strings.Contains(p.Body, "flowgraph") {
			for _, l := range strings.Split(p.Body, "\n") {
				if isFlowFence(l) {
					return true
				}
			}
		}
	}
	return false
}

// buildGraph builds the flow graph for the docs folder. It always returns a graph; the report
// says what's wrong with it.
func buildGraph(docsDir string, cfg *Config, opt graphOptions) (*Graph, *graphReport) {
	rep := &graphReport{}
	pages := loadPages(docsDir)
	rules := loadLayerRules()
	rep.LayerErrs = rules.Errs
	var comps []*Page
	for _, p := range pages {
		if p.Kind == "component" {
			comps = append(comps, p)
		}
	}
	owner := func(f string) *Page {
		for _, c := range comps {
			if matchAny(f, c.list("paths")) {
				return c
			}
		}
		return nil
	}

	// The repo's units.
	ignore := append(append(append([]string{}, cfg.Ignore...), rel(docsDir)+"/**"), rules.Ignore...)
	files := []string{}
	for _, f := range sourceFiles(ignore) {
		if codeExt[extOf(f)] && !reTestFile.MatchString(f) {
			files = append(files, f)
		}
	}
	var units []codeUnit
	for _, f := range files {
		units = append(units, extractUnits(f, fileLines(f))...)
	}
	rep.Extracted = len(units)
	rep.Granularity = opt.Granularity
	if rep.Granularity == "" {
		rep.Granularity = "symbol"
		if len(units) > maxUnits {
			rep.Granularity = "file"
		}
	}
	if rep.Granularity == "file" {
		seen := map[string]bool{}
		var perFile []codeUnit
		for _, u := range units {
			if !seen[u.File] {
				seen[u.File] = true
				perFile = append(perFile, codeUnit{File: u.File, Name: path.Base(u.File), Line: 1, End: max(1, len(fileLines(u.File)))})
			}
		}
		units = perFile
	}

	// Every unit gets an id; steps are matched to units below, adding units as needed.
	type gunit struct {
		GUnit
		cu   *codeUnit
		dist string // district key, before ids are final
	}
	all := []*gunit{}
	byID := map[string]*gunit{}
	usedIDs := map[string]bool{}
	newID := func(base string) string {
		id, n := base, 2
		for usedIDs[id] {
			id = fmt.Sprintf("%s~%d", base, n)
			n++
		}
		usedIDs[id] = true
		return id
	}
	addCode := func(cu codeUnit) *gunit {
		key := cu.File
		if cu.Symbol != "" {
			key += ":" + cu.Symbol
		} else if cu.Route != "" {
			key += ":" + cu.Route
		} else if cu.Line > 1 && rep.Granularity != "file" {
			key += ":" + strconv.Itoa(cu.Line)
		}
		g := &gunit{GUnit: GUnit{ID: newID(graphID("", key)), Name: cu.Name, Kind: "code", File: cu.File}, cu: &cu}
		if cu.Line > 0 {
			g.Lines = []int{cu.Line, max(cu.Line, cu.End)}
		}
		all = append(all, g)
		byID[g.ID] = g
		return g
	}
	byFile := map[string][]*gunit{}
	byRoute := map[string][]*gunit{}
	for _, cu := range units {
		g := addCode(cu)
		byFile[cu.File] = append(byFile[cu.File], g)
		if cu.Route != "" {
			byRoute[cu.Route] = append(byRoute[cu.Route], g)
		}
	}
	lastSeg := func(s string) string { return s[strings.LastIndexByte(s, '.')+1:] }
	resolve := func(r unitRef) (*gunit, string) {
		switch r.Kind {
		case "route":
			list := byRoute[r.Name]
			if len(list) == 0 && strings.HasPrefix(r.Name, "ANY ") { // any method on that path
				_, p, _ := strings.Cut(r.Name, " ")
				for k, l := range byRoute {
					if strings.HasSuffix(k, " "+p) {
						list = append(list, l...)
					}
				}
			}
			if len(list) == 1 {
				return list[0], ""
			}
			if len(list) > 1 {
				return nil, fmt.Sprintf("route %s is defined %d times; use a code ref instead", r.Name, len(list))
			}
			return nil, fmt.Sprintf("no route %s found in the code; use a code ref (`path#handler`) instead", r.Name)
		case "code":
			if !fileExists(filepath.Join(root, filepath.FromSlash(r.File))) {
				return nil, "no such file: " + r.File
			}
			list := byFile[r.File]
			if rep.Granularity == "file" {
				if len(list) > 0 {
					return list[0], ""
				}
				return addCode(codeUnit{File: r.File, Name: path.Base(r.File), Line: 1, End: max(1, len(fileLines(r.File)))}), ""
			}
			if r.Symbol != "" {
				var same []*gunit
				for _, g := range list {
					if g.cu.Symbol == r.Symbol {
						return g, ""
					}
					if g.cu.Symbol != "" && lastSeg(g.cu.Symbol) == lastSeg(r.Symbol) {
						same = append(same, g)
					}
				}
				if len(same) == 1 {
					return same[0], ""
				}
				from, to, ok := symbolSpan(r.File, r.Symbol)
				if !ok {
					return nil, fmt.Sprintf("no %s in %s", r.Symbol, r.File)
				}
				g := addCode(codeUnit{File: r.File, Symbol: r.Symbol, Name: r.Symbol, Line: from, End: to})
				byFile[r.File] = append(byFile[r.File], g)
				return g, ""
			}
			if r.Line > 0 {
				var best *gunit
				for _, g := range list {
					if g.cu.Line <= r.Line && r.Line <= g.cu.End && (best == nil || g.cu.End-g.cu.Line < best.cu.End-best.cu.Line) {
						best = g
					}
				}
				if best != nil {
					return best, ""
				}
				from, to := r.Line, r.End
				if to == 0 {
					from, to = lineSpan(r.File, r.Line)
				}
				if from == 0 {
					return nil, fmt.Sprintf("%s has no line %d", r.File, r.Line)
				}
				g := addCode(codeUnit{File: r.File, Name: path.Base(r.File) + ":" + strconv.Itoa(r.Line), Line: from, End: to})
				byFile[r.File] = append(byFile[r.File], g)
				return g, ""
			}
			for _, g := range list {
				if g.cu.Symbol == "" && g.cu.Route == "" && g.cu.Line <= 1 {
					return g, ""
				}
			}
			g := addCode(codeUnit{File: r.File, Name: path.Base(r.File), Line: 1, End: max(1, len(fileLines(r.File)))})
			byFile[r.File] = append(byFile[r.File], g)
			return g, ""
		}
		// store, file, queue, cache, external
		prefix := map[string]string{"store": "db.", "file": "files.", "queue": "queue.", "cache": "cache.", "external": "ext."}[r.Kind]
		key := prefix + slugName(r.Name)
		if g := byID[key]; g != nil {
			if g.Kind != r.Kind {
				return nil, fmt.Sprintf("%s %s clashes with %s %s", r.Kind, r.Name, g.Kind, g.Name)
			}
			return g, ""
		}
		g := &gunit{GUnit: GUnit{ID: key, Name: r.Name, Kind: r.Kind}}
		if r.Kind != "external" {
			d := r.District
			if d == "" {
				d = map[string]string{"store": "database", "file": "files", "queue": "queues", "cache": "cache"}[r.Kind]
			}
			g.Layer, g.dist = "data", d
		}
		usedIDs[key] = true
		all = append(all, g)
		byID[key] = g
		return g, ""
	}

	// Flows, from the blocks in the flow pages.
	var flows []GFlow
	flowIDs := map[string]bool{}
	for _, p := range flowPages(pages) {
		rep.FlowPages++
		for _, b := range flowBlocks(p) {
			rep.Blocks = append(rep.Blocks, b)
			for _, e := range b.Errs {
				rep.Problems = append(rep.Problems, Issue{"content/" + p.File, e.Line + p.Offset, "flowgraph", e.Msg})
			}
			if len(b.Errs) > 0 {
				continue
			}
			f := GFlow{ID: graphID("", b.ID), Name: b.Name, Lens: b.Lens, DocRef: p.ID, Parent: b.Parent}
			if flowIDs[f.ID] {
				rep.Problems = append(rep.Problems, Issue{"content/" + p.File, b.Line + p.Offset, "flowgraph", "another flow already has the id " + f.ID + "; add `id: …` to this block"})
				continue
			}
			flowIDs[f.ID] = true
			if b.Color != "" {
				c := strings.ToLower(b.Color)
				f.Color = &c
			}
			okAll := true
			for _, s := range b.Steps {
				g, err := resolve(s.Ref)
				if err != "" {
					rep.Problems = append(rep.Problems, Issue{"content/" + p.File, s.Line + p.Offset, "flowgraph", err})
					okAll = false
					continue
				}
				f.Steps = append(f.Steps, GStep{Unit: g.ID, Label: s.Label, Effect: s.Effect})
			}
			if !okAll {
				continue
			}
			for _, br := range b.Branches {
				f.Branches = append(f.Branches, GBranch{AtStep: br.At, Label: br.Label, Target: br.Target})
			}
			flows = append(flows, f)
		}
	}

	// Layers and districts.
	unclassified := map[string]bool{}
	layerOf := func(f string) string {
		for _, id := range rules.Order {
			if matchAny(f, rules.Globs[id]) {
				return id
			}
		}
		if c := owner(f); c != nil && c.str("layer") != "" {
			return c.str("layer")
		}
		if l := heuristicLayer(f); l != "" {
			return l
		}
		unclassified[f] = true
		return "svc"
	}
	for _, g := range all {
		if g.Kind != "code" {
			continue
		}
		g.Layer = layerOf(g.File)
		if c := owner(g.File); c != nil {
			g.dist, g.DocRef = "c:"+c.ID, c.ID
		} else {
			g.dist = "d:" + path.Dir(g.File)
		}
	}
	for f := range unclassified {
		rep.Unclassified = append(rep.Unclassified, f)
	}
	sort.Strings(rep.Unclassified)

	// Layer order: layers.yml first, then the usual four, then anything else.
	used := map[string]bool{}
	for _, g := range all {
		if g.Layer != "" {
			used[g.Layer] = true
		}
	}
	var order []string
	seen := map[string]bool{}
	push := func(id string) {
		if used[id] && !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	for _, id := range rules.Order {
		push(id)
	}
	for _, l := range defaultLayers {
		push(l.ID)
	}
	for _, id := range sortedKeys(used) {
		push(id)
	}
	if len(order) == 0 {
		order = []string{"svc"}
	}
	layerIdx := map[string]int{}
	var layers []GLayer
	for i, id := range order {
		layerIdx[id] = i
		layers = append(layers, GLayer{ID: id, Name: layerName(id), Order: i})
	}

	compTitle := map[string]string{}
	compPath := map[string]string{}
	for _, c := range comps {
		compTitle[c.ID] = c.Title
		if ps := c.list("paths"); len(ps) > 0 {
			compPath[c.ID] = ps[0]
		}
	}
	type dkey struct{ layer, key string }
	districtID := map[dkey]string{}
	var districts []GDistrict
	dnames := map[string]map[string]bool{} // layer -> names used
	for _, g := range all {
		if g.Kind == "external" {
			continue
		}
		k := dkey{g.Layer, g.dist}
		if _, ok := districtID[k]; !ok {
			var name, p string
			switch {
			case strings.HasPrefix(g.dist, "c:"):
				name, p = strings.ToLower(compTitle[g.dist[2:]]), compPath[g.dist[2:]]
			case strings.HasPrefix(g.dist, "d:"):
				p = g.dist[2:]
				name = path.Base(p)
				if p == "." {
					name = "(root)"
				}
			default:
				name = g.dist
			}
			if dnames[g.Layer] == nil {
				dnames[g.Layer] = map[string]bool{}
			}
			base, n := name, 2
			for dnames[g.Layer][name] {
				name = fmt.Sprintf("%s (%d)", base, n)
				n++
			}
			dnames[g.Layer][name] = true
			id := g.Layer + "/" + graphID("", slugName(name))
			for _, d := range districts {
				if d.ID == id {
					id += "~" + strconv.Itoa(len(districts))
				}
			}
			districtID[k] = id
			districts = append(districts, GDistrict{ID: id, Layer: g.Layer, Name: name, Path: p})
		}
		g.District = districtID[k]
	}
	sort.SliceStable(districts, func(i, j int) bool {
		a, b := districts[i], districts[j]
		if layerIdx[a.Layer] != layerIdx[b.Layer] {
			return layerIdx[a.Layer] < layerIdx[b.Layer]
		}
		return a.ID < b.ID
	})
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if (a.Kind == "external") != (b.Kind == "external") {
			return b.Kind == "external"
		}
		if layerIdx[a.Layer] != layerIdx[b.Layer] {
			return layerIdx[a.Layer] < layerIdx[b.Layer]
		}
		if a.District != b.District {
			return a.District < b.District
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if len(a.Lines) > 0 && len(b.Lines) > 0 && a.Lines[0] != b.Lines[0] {
			return a.Lines[0] < b.Lines[0]
		}
		return a.ID < b.ID
	})

	g := &Graph{Version: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Layers: layers, Districts: districts, Units: []GUnit{}, Flows: flows}
	if g.Flows == nil {
		g.Flows = []GFlow{}
	}
	if g.Districts == nil {
		g.Districts = []GDistrict{}
	}
	g.Repo = GraphRepo{Name: cfg.Project, Web: opt.Web, Root: opt.Root}
	if g.Repo.Name == "" {
		g.Repo.Name = filepath.Base(root)
	}
	if isGit() {
		if h, ok := git("rev-parse", "HEAD"); ok {
			c := strings.TrimSpace(h)
			g.Repo.Commit = &c
		}
	}
	for _, u := range all {
		g.Units = append(g.Units, u.GUnit)
	}
	if rep.Granularity == "symbol" && len(g.Flows) > 0 {
		names := map[string]string{}
		for _, u := range all {
			if u.cu != nil && u.cu.Symbol != "" {
				names[u.ID] = lastSeg(u.cu.Symbol)
			}
		}
		computeReach(g, names, sourceVisibility(getRepoCtx(docsDir)))
	}
	rep.Invalid = validateGraph(g)
	return g, rep
}

// ---------- cached graph for the server ----------

type graphCache struct {
	mu  sync.Mutex
	at  time.Time
	g   *Graph
	rep *graphReport
}

var liveGraph = &graphCache{}

func (c *graphCache) get(docsDir string) (*Graph, *graphReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.g == nil || time.Since(c.at) > 3*time.Second {
		c.g, c.rep = buildGraph(docsDir, loadConfig(docsDir), graphOptions{Root: posix(relTo(docsDir, root))})
		c.at = time.Now()
	}
	return c.g, c.rep
}

// relTo is the path from folder `from` to `to`, for repo.root.
func relTo(from, to string) string {
	r, err := filepath.Rel(from, to)
	if err != nil {
		return ".."
	}
	return r
}
