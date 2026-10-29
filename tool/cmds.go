package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func jsonUnmarshal(b []byte, v any) error {
	return json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), v)
}

// ---------- stats ----------

var reManifest = regexp.MustCompile(`(^|/)(package\.json|composer\.json|go\.mod|Cargo\.toml|pyproject\.toml|requirements\.txt|pom\.xml|build\.gradle(\.kts)?|.*\.csproj|project\.godot|Gemfile|addon\.json|CMakeLists\.txt|Makefile|vite\.config\.\w+|tsconfig\.json)$`)

func cmdStats(o options) {
	depth, _ := strconv.Atoi(o.depth)
	if depth < 1 {
		depth = 3
	}
	var ignore []string
	if d := findDocsDir(o, false); d != "" {
		ignore = append(ignore, loadConfig(d).Ignore...)
		ignore = append(ignore, rel(d)+"/**")
	}
	files := sourceFiles(ignore)
	type agg struct{ files, lines int }
	dirs, exts := map[string]*agg{}, map[string]*agg{}
	total := 0
	type sized struct {
		f string
		n int
	}
	var all []sized
	for _, f := range files {
		n := lineCount(f)
		total += n
		all = append(all, sized{f, n})
		segs := strings.Split(f, "/")
		for d := 1; d <= depth && d <= len(segs)-1; d++ {
			key := strings.Join(segs[:d], "/") + "/"
			if dirs[key] == nil {
				dirs[key] = &agg{}
			}
			dirs[key].files++
			dirs[key].lines += n
		}
		e := extOf(f)
		if e == "" {
			e = "(none)"
		}
		if exts[e] == nil {
			exts[e] = &agg{}
		}
		exts[e].files++
		exts[e].lines += n
	}
	var b strings.Builder
	head := " (not a git repo)"
	if isGit() {
		br, _ := git("rev-parse", "--abbrev-ref", "HEAD")
		sh, _ := git("rev-parse", "--short", "HEAD")
		head = fmt.Sprintf(" (git: %s @ %s)", br, sh)
	}
	fmt.Fprintf(&b, "# Repo inventory: %d text files, %d lines%s\n\n## By extension\n", len(files), total, head)
	ek := sortedKeys(exts)
	sort.SliceStable(ek, func(i, j int) bool { return exts[ek[i]].lines > exts[ek[j]].lines })
	for i, e := range ek {
		if i == 15 {
			break
		}
		fmt.Fprintf(&b, "- %s: %d files, %d lines\n", e, exts[e].files, exts[e].lines)
	}
	fmt.Fprintf(&b, "\n## Directories (depth %d)\n", depth)
	for _, d := range sortedKeys(dirs) {
		fmt.Fprintf(&b, "%s- %s  (%d files, %d lines)\n", strings.Repeat("  ", strings.Count(d, "/")-1), d, dirs[d].files, dirs[d].lines)
	}
	var rootFiles []string
	for _, s := range all {
		if !strings.Contains(s.f, "/") {
			rootFiles = append(rootFiles, fmt.Sprintf("%s (%d)", s.f, s.n))
		}
	}
	if len(rootFiles) > 0 {
		fmt.Fprintf(&b, "\n## Root files\n%s\n", strings.Join(rootFiles, ", "))
	}
	b.WriteString("\n## Manifests / config\n")
	for _, f := range files {
		if reManifest.MatchString(f) {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("\n## Largest files\n")
	sort.SliceStable(all, func(i, j int) bool { return all[i].n > all[j].n })
	for i, s := range all {
		if i == 15 {
			break
		}
		fmt.Fprintf(&b, "- %s: %d\n", s.f, s.n)
	}
	if isGit() {
		if churn, ok := git("log", "--since=6 months ago", "--name-only", "--pretty=format:"); ok && churn != "" {
			inSet := map[string]bool{}
			for _, f := range files {
				inSet[f] = true
			}
			counts := map[string]int{}
			for _, f := range lines(churn) {
				if inSet[f] {
					counts[f]++
				}
			}
			ck := sortedKeys(counts)
			sort.SliceStable(ck, func(i, j int) bool { return counts[ck[i]] > counts[ck[j]] })
			if len(ck) > 0 {
				b.WriteString("\n## Most changed (6 months)\n")
				for i, f := range ck {
					if i == 10 {
						break
					}
					fmt.Fprintf(&b, "- %s: %d commits\n", f, counts[f])
				}
			}
		}
	}
	fmt.Print(b.String())
}

// ---------- install-viewer ----------

func readAsset(name string) []byte {
	b, err := assets.ReadFile("assets/" + name)
	if err != nil {
		die("missing embedded asset " + name)
	}
	return b
}

func gunzip(b []byte) []byte {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		die(err.Error())
	}
	out, err := io.ReadAll(r)
	if err != nil {
		die(err.Error())
	}
	return out
}

// skillDir is the folder holding SKILL.md: the binary lives in <skill>/bin.
func skillDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	d := filepath.Dir(exe)
	if filepath.Base(d) == "bin" {
		return filepath.Dir(d)
	}
	return d
}

// guidanceVersion is the version of the page-writing guidance (skills/codedocs/reference/VERSION).
// Pages stamped with an older one are "outdated"; tool-only releases don't change it.
func guidanceVersion() string {
	if sd := skillDir(); sd != "" {
		if b, err := os.ReadFile(filepath.Join(sd, "reference", "VERSION")); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	return version
}

// defaultTheme is the theme new docs folders start with: the user's own default if they saved
// one (/codedocs style), otherwise the built-in theme. The user's default lives in their config
// folder so plugin updates, which replace the skill folder, don't lose it; the skill folder is
// still checked for defaults saved by 0.4 and earlier.
func defaultTheme() ([]byte, string) {
	if cd, err := os.UserConfigDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(cd, "codedocs", "default-theme.css")); err == nil {
			return b, "your default-theme.css"
		}
	}
	if sd := skillDir(); sd != "" {
		if b, err := os.ReadFile(filepath.Join(sd, "default-theme.css")); err == nil {
			return b, "your default-theme.css"
		}
	}
	return readAsset("theme.css"), "built-in default"
}

func cmdInstallViewer(o options) {
	docsDir := findDocsDir(o, false)
	if docsDir == "" {
		target := ""
		if len(o.pos) > 0 {
			target = o.pos[0]
		} else {
			// docs/ unless it already holds other documentation; then docs/codebase/.
			target = "docs"
			ours := set("index.html", "viewer.css", "theme.css", "docs.config.json", "manifest.json", "content", "lib")
			if entries, err := os.ReadDir(filepath.Join(root, "docs")); err == nil {
				for _, e := range entries {
					if !ours[e.Name()] {
						target = "docs/codebase"
						break
					}
				}
			}
		}
		docsDir = filepath.Join(root, filepath.FromSlash(target))
	}
	if err := os.MkdirAll(filepath.Join(docsDir, "content"), 0o755); err != nil {
		die(err.Error())
	}
	var actions []string
	write := func(name string, b []byte) {
		p := filepath.Join(docsDir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			die(err.Error())
		}
	}
	write("index.html", readAsset("index.html"))
	write("viewer.css", readAsset("viewer.css"))
	actions = append(actions, "wrote index.html, viewer.css")
	if !fileExists(filepath.Join(docsDir, "theme.css")) {
		b, from := defaultTheme()
		write("theme.css", b)
		actions = append(actions, "created theme.css ("+from+")")
	} else {
		actions = append(actions, "kept theme.css")
	}
	if o.offline {
		entries, _ := assets.ReadDir("assets/lib")
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".gz") {
				write("lib/"+strings.TrimSuffix(e.Name(), ".gz"), gunzip(readAsset("lib/"+e.Name())))
			}
		}
		actions = append(actions, "wrote lib/ (viewer works offline without the server)")
	}
	cfgFile := filepath.Join(docsDir, "docs.config.json")
	var cfg *Config
	if fileExists(cfgFile) {
		cfg = loadConfig(docsDir)
		prev := cfg.SkillVersion
		cfg.SkillVersion = version
		if o.project != "" {
			cfg.Project = o.project
		}
		cfg.RepoRootAbs = posix(root)
		actions = append(actions, fmt.Sprintf("updated docs.config.json (skill_version %s -> %s)", prev, version))
	} else {
		project := o.project
		if project == "" {
			project = filepath.Base(root)
		}
		cfg = &Config{SkillVersion: version, Project: project, RepoRootAbs: posix(root), Editor: "vscode",
			Ignore: []string{}, Created: time.Now().Format("2006-01-02")}
		actions = append(actions, "created docs.config.json")
	}
	writeJSON(cfgFile, cfg)
	if len(loadPages(docsDir)) > 0 {
		writeJSON(filepath.Join(docsDir, "manifest.json"), buildManifest(docsDir, cfg))
		actions = append(actions, "rebuilt manifest.json")
	}
	fmt.Printf("docs folder: %s\n- %s\n", rel(docsDir), strings.Join(actions, "\n- "))
}

// ---------- manifest ----------

func cmdManifest(o options) {
	docsDir := findDocsDir(o, true)
	m := buildManifest(docsDir, loadConfig(docsDir))
	writeJSON(filepath.Join(docsDir, "manifest.json"), m)
	counts := map[string]int{}
	var order []string
	for _, p := range m.Pages {
		if counts[p.Kind] == 0 {
			order = append(order, p.Kind)
		}
		counts[p.Kind]++
	}
	var parts []string
	for _, k := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	fmt.Printf("manifest.json: %d pages (%s), %d symbols resolved\n", len(m.Pages), strings.Join(parts, ", "), len(m.Symbols))
}

// ---------- check ----------

type Issue struct {
	Page   string `json:"page"`
	Line   int    `json:"line"`
	Type   string `json:"type"`
	Detail string `json:"detail"`
}

func cmdCheck(o options) {
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	pages := loadPages(docsDir)
	issues := []Issue{}
	addBody := func(p *Page, docLine int, typ, detail string) {
		issues = append(issues, Issue{"content/" + p.File, docLine + p.Offset, typ, detail})
	}
	addMeta := func(p *Page, typ, detail string) { issues = append(issues, Issue{"content/" + p.File, 1, typ, detail}) }
	lineCache := map[string]int{}
	nLines := func(f string) int {
		if n, ok := lineCache[f]; ok {
			return n
		}
		lineCache[f] = lineCount(f)
		return lineCache[f]
	}
	ids := map[string]bool{}
	compSlugs := map[string]bool{}
	for _, p := range pages {
		ids[p.File] = true
		if p.Kind == "component" {
			compSlugs[p.slug()] = true
		}
	}
	for _, p := range pages {
		for _, r := range extractRefs(p.Body) {
			if !fileExists(filepath.Join(root, filepath.FromSlash(r.File))) {
				addBody(p, r.DocLine, "missing-file", r.Raw)
				continue
			}
			if r.Line > 0 {
				if n := nLines(r.File); r.Line > n || r.End > n {
					addBody(p, r.DocLine, "line-out-of-range", fmt.Sprintf("%s (file has %d lines)", r.Raw, n))
				}
			}
			if r.Symbol != "" && resolveSymbol(r.File, r.Symbol) == 0 {
				addBody(p, r.DocLine, "missing-symbol", r.Raw)
			}
		}
		for _, l := range extractLinks(p.Body) {
			target := path.Clean(path.Join(path.Dir(p.File), l.Href))
			if !ids[target] {
				addBody(p, l.DocLine, "broken-link", l.Href)
			}
		}
		for _, c := range p.list("components") {
			if !compSlugs[c] {
				addMeta(p, "unknown-component", c)
			}
		}
		if p.Kind == "component" && len(p.list("paths")) == 0 {
			addMeta(p, "component-without-paths", p.ID)
		}
	}
	var compGlobs []string
	for _, p := range pages {
		if p.Kind == "component" {
			compGlobs = append(compGlobs, p.list("paths")...)
		}
	}
	files := sourceFiles(append(append([]string{}, cfg.Ignore...), rel(docsDir)+"/**"))
	codeFiles := []string{}
	for _, f := range files {
		if codeExt[extOf(f)] {
			codeFiles = append(codeFiles, f)
		}
	}
	uncovered := []string{}
	if len(compGlobs) > 0 {
		for _, f := range codeFiles {
			if !matchAny(f, compGlobs) {
				uncovered = append(uncovered, f)
			}
		}
	}
	byDir := map[string]int{}
	for _, f := range uncovered {
		d := "(root)"
		if strings.Contains(f, "/") {
			s := strings.Split(f, "/")
			d = strings.Join(s[:min(2, len(s)-1)], "/")
		}
		byDir[d]++
	}
	outdated := []string{}
	gv := guidanceVersion()
	for _, p := range pages {
		if g := p.str("generated_with"); g != "" && semverCmp(g, gv) < 0 {
			outdated = append(outdated, p.ID)
		}
	}
	if o.json {
		sample := uncovered
		if len(sample) > 50 {
			sample = sample[:50]
		}
		printJSON(map[string]any{
			"ok": len(issues) == 0, "issues": issues, "outdated": outdated,
			"uncovered": map[string]any{"count": len(uncovered), "total": len(codeFiles), "byDir": byDir, "files": sample},
		})
	} else {
		if len(issues) > 0 {
			fmt.Printf("%d issue(s):\n", len(issues))
		} else {
			fmt.Println("No broken refs or links.")
		}
		for _, i := range issues {
			fmt.Printf("- %s:%d  %s  %s\n", i.Page, i.Line, i.Type, i.Detail)
		}
		if len(uncovered) > 0 {
			fmt.Printf("\nCoverage: %d of %d code files are not claimed by any component's paths:\n", len(uncovered), len(codeFiles))
			dk := sortedKeys(byDir)
			sort.SliceStable(dk, func(i, j int) bool { return byDir[dk[i]] > byDir[dk[j]] })
			for i, d := range dk {
				if i == 12 {
					break
				}
				fmt.Printf("- %s: %d\n", d, byDir[d])
			}
		}
		if len(outdated) > 0 {
			fmt.Printf("\n%d page(s) written with older page guidance (current %s): %s\n", len(outdated), gv, strings.Join(outdated, ", "))
		}
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
}

// ---------- affected ----------

type changedFile struct {
	File       string   `json:"file"`
	Exists     bool     `json:"exists"`
	Components []string `json:"components"`
}
type pageHit struct {
	Reason string `json:"reason"`
	Locked bool   `json:"locked"`
}

func cmdAffected(o options) {
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	if !isGit() {
		die("affected needs a git repo")
	}
	rng := ""
	if len(o.pos) > 0 {
		rng = o.pos[0]
	}
	base := rng
	if base == "" && cfg.LastSynced != nil {
		base = *cfg.LastSynced
	}
	if base == "" {
		die("no last_synced commit in docs.config.json; pass a range, e.g. HEAD~3 or main..feature")
	}
	var changed0 []string
	args := []string{"diff", "--name-only", base, "HEAD"}
	if strings.Contains(base, "..") {
		args = []string{"diff", "--name-only", base}
	}
	out, ok := git(args...)
	if !ok {
		die(`git could not resolve range "` + base + `"`)
	}
	changed0 = append(changed0, lines(out)...)
	if rng == "" {
		for _, a := range [][]string{{"diff", "--name-only"}, {"diff", "--name-only", "--cached"}, {"ls-files", "--others", "--exclude-standard"}} {
			o2, _ := git(a...)
			changed0 = append(changed0, lines(o2)...)
		}
	}
	docsRel := rel(docsDir) + "/"
	ignore := append(append([]string{}, defaultIgnore...), cfg.Ignore...)
	var changed []string
	for _, f := range uniq(changed0) {
		if f != "" && !strings.HasPrefix(f, docsRel) && !matchAny(f, ignore) {
			changed = append(changed, f)
		}
	}
	pages := loadPages(docsDir)
	head, _ := git("rev-parse", "--short", "HEAD")
	compFiles := map[string][]string{}
	res := struct {
		Base       string              `json:"base"`
		Head       string              `json:"head"`
		Changed    []changedFile       `json:"changed"`
		Components map[string][]string `json:"components"`
		Pages      map[string]pageHit  `json:"pages"`
		Unclaimed  []string            `json:"unclaimed"`
	}{base, head, []changedFile{}, compFiles, map[string]pageHit{}, []string{}}
	for _, f := range changed {
		owners := []string{}
		for _, c := range pages {
			if c.Kind == "component" && matchAny(f, c.list("paths")) {
				owners = append(owners, c.ID)
				compFiles[c.ID] = append(compFiles[c.ID], f)
			}
		}
		res.Changed = append(res.Changed, changedFile{f, fileExists(filepath.Join(root, filepath.FromSlash(f))), owners})
		if len(owners) == 0 {
			res.Unclaimed = append(res.Unclaimed, f)
		}
	}
	var pageOrder []string
	for _, p := range pages {
		refFiles := map[string]bool{}
		for _, r := range extractRefs(p.Body) {
			refFiles[r.File] = true
		}
		var hits, comps []string
		for _, f := range changed {
			if refFiles[f] {
				hits = append(hits, f)
			}
		}
		for _, s := range p.list("components") {
			if _, ok := compFiles["components/"+s]; ok {
				comps = append(comps, "components/"+s)
			}
		}
		_, owns := compFiles[p.ID]
		if len(hits) > 0 || len(comps) > 0 || owns {
			var reason []string
			if len(hits) > 0 {
				reason = append(reason, "references "+strings.Join(hits, ", "))
			}
			if len(comps) > 0 {
				reason = append(reason, "involves "+strings.Join(comps, ", "))
			}
			if owns {
				reason = append(reason, "owns changed files")
			}
			res.Pages[p.ID] = pageHit{strings.Join(reason, "; "), p.flag("locked")}
			pageOrder = append(pageOrder, p.ID)
		}
	}
	if o.json {
		printJSON(res)
		return
	}
	since := ""
	if base != "" {
		since = "since " + base + " "
	}
	fmt.Printf("Changes %s(HEAD %s): %d file(s)\n", since, head, len(changed))
	for _, c := range res.Changed {
		del := ""
		if !c.Exists {
			del = " (deleted)"
		}
		owner := "  -> (no component)"
		if len(c.Components) > 0 {
			owner = "  -> " + strings.Join(c.Components, ", ")
		}
		fmt.Printf("- %s%s%s\n", c.File, del, owner)
	}
	fmt.Printf("\nPages to review (%d):\n", len(res.Pages))
	for _, id := range pageOrder {
		h := res.Pages[id]
		lock := ""
		if h.Locked {
			lock = " [locked]"
		}
		fmt.Printf("- %s%s: %s\n", id, lock, h.Reason)
	}
	if len(res.Unclaimed) > 0 {
		fmt.Printf("\nUnclaimed files (new area? extend a component's paths or add a component): %s\n", strings.Join(res.Unclaimed, ", "))
	}
}

// ---------- mark-synced / status ----------

func cmdMarkSynced(o options) {
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	if !isGit() {
		die("mark-synced needs a git repo")
	}
	h, _ := git("rev-parse", "HEAD")
	cfg.LastSynced = &h
	cfg.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	writeJSON(filepath.Join(docsDir, "docs.config.json"), cfg)
	fmt.Printf("last_synced = %s\n", h[:min(10, len(h))])
}

func cmdStatus(o options) {
	docsDir := findDocsDir(o, false)
	if docsDir == "" {
		fmt.Printf("No docs in this repo yet (codedocs %s). Run /codedocs init.\n", version)
		return
	}
	cfg := loadConfig(docsDir)
	pages := loadPages(docsDir)
	counts := map[string]int{}
	var outdated, locked []string
	gv := guidanceVersion()
	for _, p := range pages {
		counts[p.Kind]++
		if g := p.str("generated_with"); g == "" || semverCmp(g, gv) < 0 {
			if g == "" {
				g = "?"
			}
			outdated = append(outdated, p.ID+"@"+g)
		}
		if p.flag("locked") {
			locked = append(locked, p.ID)
		}
	}
	var cs []string
	for _, k := range sortedKeys(counts) {
		cs = append(cs, fmt.Sprintf("%d %s", counts[k], k))
	}
	fmt.Printf("Docs folder: %s\nProject: %s\n", rel(docsDir), cfg.Project)
	up := ""
	if semverCmp(cfg.SkillVersion, version) < 0 {
		up = "  -> viewer upgrade available (/codedocs upgrade)"
	}
	fmt.Printf("Skill version: installed %s (page guidance %s), docs created/upgraded with %s%s\n", version, gv, cfg.SkillVersion, up)
	fmt.Printf("Pages: %d (%s)\n", len(pages), strings.Join(cs, ", "))
	if len(outdated) > 0 {
		fmt.Printf("Pages written with older page guidance: %d (%s)\n", len(outdated), strings.Join(outdated, ", "))
	} else {
		fmt.Println("Pages written with older page guidance: 0")
	}
	if len(locked) > 0 {
		fmt.Printf("Locked pages: %s\n", strings.Join(locked, ", "))
	}
	if isGit() {
		if cfg.LastSynced != nil && *cfg.LastSynced != "" {
			n, ok := git("rev-list", "--count", *cfg.LastSynced+"..HEAD")
			if !ok {
				n = "?"
			}
			s := *cfg.LastSynced
			fmt.Printf("Last synced: %s (%s commit(s) behind HEAD)\n", s[:min(10, len(s))], n)
		} else {
			fmt.Println("Last synced: never")
		}
	}
}
