package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- docs folder + config ----------

type Config struct {
	SkillVersion string      `json:"skill_version"`
	Project      string      `json:"project"`
	RepoRootAbs  string      `json:"repo_root_abs"`
	Editor       string      `json:"editor"`
	Ignore       []string    `json:"ignore"`
	LastSynced   *string     `json:"last_synced"`
	LastSyncedAt string      `json:"last_synced_at,omitempty"`
	Created      string      `json:"created,omitempty"`
	Links        *LinkConfig `json:"links,omitempty"`
}

// LinkConfig holds per-repo rules for code-panel links, written by /codedocs init.
type LinkConfig struct {
	Aliases map[string]string `json:"aliases,omitempty"` // "@/": "src/", "App\": "src/" (PHP namespaces contain a backslash)
	Roots   []string          `json:"roots,omitempty"`   // extra source roots for bare imports, Python and PHP
}

func findDocsDir(o options, required bool) string {
	if o.dir != "" {
		return filepath.Join(root, filepath.FromSlash(o.dir))
	}
	for _, c := range []string{"docs", "docs/codebase", "codebase-docs"} {
		d := filepath.Join(root, filepath.FromSlash(c))
		if fileExists(filepath.Join(d, "docs.config.json")) {
			return d
		}
	}
	if required {
		die("no docs folder found (looked for docs/docs.config.json, docs/codebase/docs.config.json). Run /codedocs init first or pass --dir.")
	}
	return ""
}

func loadConfig(docsDir string) *Config {
	f := filepath.Join(docsDir, "docs.config.json")
	b, err := os.ReadFile(f)
	if err != nil {
		die("missing " + rel(f))
	}
	c := &Config{}
	if err := jsonUnmarshal(b, c); err != nil {
		die("can't read " + rel(f) + ": " + err.Error())
	}
	if c.Ignore == nil {
		c.Ignore = []string{}
	}
	return c
}

// ---------- pages ----------

type Page struct {
	ID, File, Kind, Title string
	Data                  map[string]any
	Body                  string
	Offset                int // lines of frontmatter before the body
}

func (p *Page) str(key string) string {
	switch v := p.Data[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func (p *Page) list(key string) []string {
	switch v := p.Data[key].(type) {
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return []string{}
}

func (p *Page) flag(key string) bool { b, _ := p.Data[key].(bool); return b }

func (p *Page) slug() string {
	i := strings.LastIndexByte(p.ID, '/')
	return p.ID[i+1:]
}

var (
	reFrontmatter = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---[ \t]*(?:\r?\n|\z)`)
	reKV          = regexp.MustCompile(`^([A-Za-z_][\w-]*):\s*(.*)$`)
	reNumber      = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	reQuotes      = regexp.MustCompile(`^["']|["']$`)
)

func parseFrontmatter(text string) (map[string]any, string, int) {
	data := map[string]any{}
	m := reFrontmatter.FindStringSubmatchIndex(text)
	if m == nil {
		return data, text, 0
	}
	for _, line := range strings.Split(text[m[2]:m[3]], "\n") {
		kv := reKV.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if kv == nil {
			continue
		}
		v := strings.TrimSpace(kv[2])
		switch {
		case strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]"):
			items := []string{}
			for _, s := range strings.Split(v[1:len(v)-1], ",") {
				s = reQuotes.ReplaceAllString(strings.TrimSpace(s), "")
				if s != "" {
					items = append(items, s)
				}
			}
			data[kv[1]] = items
		case v == "true" || v == "false":
			data[kv[1]] = v == "true"
		case reNumber.MatchString(v):
			f, _ := strconv.ParseFloat(v, 64)
			data[kv[1]] = f
		default:
			data[kv[1]] = reQuotes.ReplaceAllString(v, "")
		}
	}
	m0 := text[m[0]:m[1]]
	return data, text[m[1]:], strings.Count(m0, "\n")
}

// stripCode blanks fenced code blocks (refs inside ``` are examples, not refs), keeping line numbers.
func stripCode(body string) string {
	ls := strings.Split(body, "\n")
	out := make([]string, len(ls))
	copy(out, ls)
	fence, start := "", -1
	for i, l := range ls {
		if fence == "" {
			if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
				fence, start = l[:3], i
			}
			continue
		}
		if strings.HasPrefix(l, fence) {
			for j := start; j <= i; j++ {
				out[j] = strings.Repeat(" ", len(ls[j]))
			}
			fence, start = "", -1
		}
	}
	return strings.Join(out, "\n")
}

type Ref struct {
	Raw, File, Symbol  string
	Line, End, DocLine int
}

var (
	reRef     = regexp.MustCompile("`([A-Za-z0-9_.\\-/@~+]+\\.[A-Za-z0-9]+)(?::(\\d+)(?:-(\\d+))?)?(?:#([A-Za-z_$][\\w$.]*))?`")
	reLink    = regexp.MustCompile(`\]\(([^)\s]+\.md)(#[^)\s]*)?\)`)
	reScheme  = regexp.MustCompile(`(?i)^[a-z]+:`)
	reHeading = regexp.MustCompile(`^(#{2,3})\s+(.+?)\s*#*\s*$`)
	reH1      = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reMarkup  = regexp.MustCompile("[`*_]")
)

func extractRefs(body string) []Ref {
	var refs []Ref
	for i, ln := range strings.Split(stripCode(body), "\n") {
		for _, m := range reRef.FindAllStringSubmatch(ln, -1) {
			file := strings.TrimPrefix(m[1], "./")
			if !strings.Contains(file, "/") && !refExt[extOf(file)] {
				continue
			}
			r := Ref{Raw: m[0][1 : len(m[0])-1], File: file, Symbol: m[4], DocLine: i + 1}
			r.Line, _ = strconv.Atoi(m[2])
			r.End, _ = strconv.Atoi(m[3])
			refs = append(refs, r)
		}
	}
	return refs
}

type DocLink struct {
	Href    string
	DocLine int
}

func extractLinks(body string) []DocLink {
	var out []DocLink
	for i, ln := range strings.Split(stripCode(body), "\n") {
		for _, m := range reLink.FindAllStringSubmatch(ln, -1) {
			if reScheme.MatchString(m[1]) {
				continue
			}
			out = append(out, DocLink{m[1], i + 1})
		}
	}
	return out
}

type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

func headings(body string) []Heading {
	out := []Heading{}
	for _, ln := range strings.Split(stripCode(body), "\n") {
		if m := reHeading.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			out = append(out, Heading{len(m[1]), reMarkup.ReplaceAllString(m[2], "")})
		}
	}
	return out
}

func firstHeading(body string) string {
	if m := reH1.FindStringSubmatch(body); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

var folderKind = map[string]string{"components": "component", "flows": "flow", "concerns": "concern", "decisions": "decision", "changes": "change", "topics": "topic"}

func loadPages(docsDir string) []*Page {
	contentDir := filepath.Join(docsDir, "content")
	var pages []*Page
	filepath.WalkDir(contentDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		r, _ := filepath.Rel(contentDir, p)
		r = posix(r)
		text, err := readText(p)
		if err != nil {
			return nil
		}
		data, body, offset := parseFrontmatter(text)
		pg := &Page{ID: strings.TrimSuffix(r, ".md"), File: r, Data: data, Body: body, Offset: offset}
		pg.Kind = pg.str("kind")
		if pg.Kind == "" {
			pg.Kind = folderKind[strings.Split(r, "/")[0]]
		}
		if pg.Kind == "" {
			switch pg.ID {
			case "overview", "glossary", "gotchas":
				pg.Kind = pg.ID
			default:
				pg.Kind = "page"
			}
		}
		pg.Title = pg.str("title")
		if pg.Title == "" {
			pg.Title = firstHeading(body)
		}
		if pg.Title == "" {
			pg.Title = pg.ID
		}
		pages = append(pages, pg)
		return nil
	})
	return pages
}

// ---------- symbols ----------

const defKeywords = `(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\*?|class|interface|type|enum|struct|const|let|var|def|func|fn|trait|impl|record|module|namespace|public|private|protected|static|final|abstract|readonly|override|virtual)`

type symRes struct{ strong, word, def *regexp.Regexp }

// Lines that mention a name without defining it.
var reNotDef = regexp.MustCompile(`^\s*(?:namespace|package|use|import|from|require|#include)\b`)

var symCache sync.Map
var fileCache sync.Map

func symbolRegexps(part string) symRes {
	if v, ok := symCache.Load(part); ok {
		return v.(symRes)
	}
	esc := regexp.QuoteMeta(part)
	r := symRes{
		strong: regexp.MustCompile(`(?:^|[^\w$.])(?:class|interface|trait|enum|struct|type|function\*?|func|def|fn|record|class_name)\s+(?:\([^)]*\)\s*)?` + esc + `(?:[^\w$]|$)`),
		word:   regexp.MustCompile(`(?:^|[^\w$])` + esc + `(?:[^\w$]|$)`),
		// A keyword must stand alone and be followed by a space, so HTML like class="…" or a
		// name like constructor doesn't count as a declaration.
		def: regexp.MustCompile(`(?:^|[^\w$.-])` + defKeywords + `\s(?:.*?[^\w$])??` + esc + `(?:[^\w$]|$)` +
			`|(?:^|[^\w$.])` + esc + `\s*[:=(]\s*(?:async\s*)?(?:function|\(|[\w$]+\s*=>)` +
			`|^\s*(?:async\s+)?` + esc + `\s*\([^)]*\)\s*[{:]`),
	}
	symCache.Store(part, r)
	return r
}

// resolveSymbol finds the line defining (or first mentioning) a symbol; supports Class.method. 0 = not found.
func resolveSymbol(file, symbol string) int {
	var text string
	if v, ok := fileCache.Load(file); ok {
		text = v.(string)
	} else {
		t, err := readText(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return 0
		}
		text = t
		fileCache.Store(file, t)
	}
	ls := strings.Split(text, "\n")
	start, found := 0, -1
	for _, part := range strings.Split(symbol, ".") {
		re := symbolRegexps(part)
		hit := -1
		for i := start; i < len(ls); i++ {
			if re.strong.MatchString(ls[i]) && !reNotDef.MatchString(ls[i]) {
				hit = i
				break
			}
		}
		for i := start; hit == -1 && i < len(ls); i++ {
			if re.def.MatchString(ls[i]) && !reNotDef.MatchString(ls[i]) {
				hit = i
				break
			}
		}
		if hit == -1 {
			for i := start; i < len(ls); i++ {
				if re.word.MatchString(ls[i]) {
					hit = i
					break
				}
			}
		}
		if hit == -1 {
			return 0
		}
		found, start = hit, hit
	}
	return found + 1
}

// ---------- manifest ----------

type Entry struct {
	ID            string    `json:"id"`
	File          string    `json:"file"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	Order         float64   `json:"order"`
	Headings      []Heading `json:"headings"`
	Status        string    `json:"status,omitempty"`
	GeneratedWith string    `json:"generated_with,omitempty"`
	Updated       string    `json:"updated,omitempty"`
	Locked        bool      `json:"locked,omitempty"`
	Audience      string    `json:"audience,omitempty"`
	Refs          []string  `json:"refs"`
	Paths         *[]string `json:"paths,omitempty"`
	Flows         *[]string `json:"flows,omitempty"`
	Concerns      *[]string `json:"concerns,omitempty"`
	Decisions     *[]string `json:"decisions,omitempty"`
	Components    *[]string `json:"components,omitempty"`
	Topics        *[]string `json:"topics,omitempty"`
}

type Manifest struct {
	Project      string         `json:"project"`
	SkillVersion string         `json:"skill_version"`
	ToolVersion  string         `json:"tool_version"`
	RepoRootAbs  string         `json:"repo_root_abs"`
	Editor       string         `json:"editor"`
	Generated    string         `json:"generated"`
	LastSynced   *string        `json:"last_synced"`
	Pages        []*Entry       `json:"pages"`
	Symbols      map[string]int `json:"symbols"`
	Static       bool           `json:"static,omitempty"` // a codedocs export: the code panel reads code/
}

var kindOrder = []string{"overview", "map", "topic", "component", "flow", "concern", "glossary", "decision", "gotchas", "change", "page"}

func kindIndex(k string) int {
	for i, x := range kindOrder {
		if x == k {
			return i
		}
	}
	return -1
}

func buildManifest(docsDir string, cfg *Config) *Manifest {
	fileCache = sync.Map{}
	pages := loadPages(docsDir)
	comps := map[string]*Page{}
	for _, p := range pages {
		if p.Kind == "component" {
			comps[p.slug()] = p
		}
	}
	type rev struct{ flows, concerns, decisions, topics []string }
	reverse := map[string]*rev{}
	for _, p := range pages {
		for _, c := range p.list("components") {
			if comps[c] == nil {
				continue
			}
			r := reverse[c]
			if r == nil {
				r = &rev{[]string{}, []string{}, []string{}, []string{}}
				reverse[c] = r
			}
			switch p.Kind {
			case "flow":
				r.flows = append(r.flows, p.ID)
			case "concern":
				r.concerns = append(r.concerns, p.ID)
			case "topic":
				r.topics = append(r.topics, p.ID)
			case "decision":
				r.decisions = append(r.decisions, p.ID)
			}
		}
	}
	symbols := map[string]int{}
	entries := []*Entry{}
	for _, p := range pages {
		refs := extractRefs(p.Body)
		refFiles := []string{}
		for _, r := range refs {
			refFiles = append(refFiles, r.File)
			key := r.File + "#" + r.Symbol
			if r.Symbol != "" && symbols[key] == 0 && fileExists(filepath.Join(root, filepath.FromSlash(r.File))) {
				if ln := resolveSymbol(r.File, r.Symbol); ln > 0 {
					symbols[key] = ln
				}
			}
		}
		order := 999.0
		if v, ok := p.Data["order"].(float64); ok {
			order = v
		}
		e := &Entry{
			ID: p.ID, File: p.File, Kind: p.Kind, Title: p.Title, Summary: p.str("summary"), Order: order,
			Headings: headings(p.Body), Status: p.str("status"), GeneratedWith: p.str("generated_with"),
			Updated: p.str("updated"), Locked: p.flag("locked"), Audience: p.str("audience"), Refs: uniq(refFiles),
		}
		if p.Kind == "component" {
			paths := p.list("paths")
			e.Paths = &paths
			r := reverse[p.slug()]
			if r == nil {
				r = &rev{[]string{}, []string{}, []string{}, []string{}}
			}
			e.Flows, e.Concerns, e.Decisions, e.Topics = &r.flows, &r.concerns, &r.decisions, &r.topics
		}
		if p.Kind == "flow" || p.Kind == "concern" || p.Kind == "decision" || p.Kind == "topic" {
			ids := []string{}
			for _, c := range p.list("components") {
				if comps[c] != nil {
					ids = append(ids, comps[c].ID)
				}
			}
			e.Components = &ids
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if ka, kb := kindIndex(a.Kind), kindIndex(b.Kind); ka != kb {
			return ka < kb
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		la, lb := strings.ToLower(a.Title), strings.ToLower(b.Title)
		if la != lb {
			return la < lb
		}
		return a.Title < b.Title
	})
	editor := cfg.Editor
	if editor == "" {
		editor = "none"
	}
	return &Manifest{
		Project: cfg.Project, SkillVersion: cfg.SkillVersion, ToolVersion: version, RepoRootAbs: cfg.RepoRootAbs,
		Editor: editor, Generated: time.Now().UTC().Format(time.RFC3339), LastSynced: cfg.LastSynced,
		Pages: entries, Symbols: symbols,
	}
}
