package main

// `codedocs flows`: write the flow graph and the flow view into the docs folder.
// `codedocs flows --scan`: what the agent needs before writing flowgraph blocks.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// writeFlowView puts the flow view next to the docs: flowgraph.json (the data), flowgraph.js
// (the same, for pages opened from disk, where fetch() fails), flowview.html and .css, and
// lib/flowview.js. Nothing in it needs a server.
func writeFlowView(dir string, g *Graph) {
	write := func(name string, b []byte) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			die(err.Error())
		}
	}
	write("flowgraph.json", marshal(g))
	write("flowgraph.js", flowgraphJS(g))
	write("flowview.html", readAsset("flowview.html"))
	write("flowview.css", readAsset("flowview.css"))
	write("lib/flowview.js", gunzip(readAsset("lib/flowview.js.gz")))
}

func flowgraphJS(g *Graph) []byte {
	return []byte("window.__FLOWGRAPH__ = " + string(compactJSON(g)) + ";\n")
}

func printProblems(rep *graphReport) {
	for _, e := range rep.LayerErrs {
		fmt.Println("- " + e)
	}
	for _, i := range rep.Problems {
		fmt.Printf("- %s:%d  %s\n", i.Page, i.Line, i.Detail)
	}
	for _, e := range rep.Invalid {
		fmt.Println("- flowgraph.json: " + e)
	}
}

func coverage(g *Graph) (covered int) {
	on := map[string]bool{}
	for _, f := range g.Flows {
		for _, s := range f.Steps {
			on[s.Unit] = true
		}
	}
	return len(on)
}

func cmdFlows(o options) {
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	if o.granularity != "" && o.granularity != "symbol" && o.granularity != "file" {
		die("--granularity is symbol or file")
	}
	if o.scan {
		cmdFlowsScan(o, docsDir, cfg)
		return
	}
	g, rep := buildGraph(docsDir, cfg, graphOptions{Granularity: o.granularity, Root: posix(relTo(docsDir, root))})
	if o.json {
		printJSON(map[string]any{"ok": rep.ok(), "problems": rep.Problems, "invalid": rep.Invalid, "layerErrors": rep.LayerErrs,
			"layers": len(g.Layers), "units": len(g.Units), "flows": len(g.Flows), "covered": coverage(g), "granularity": rep.Granularity,
			"unclassified": rep.Unclassified})
	}
	if !rep.ok() {
		if !o.json {
			fmt.Printf("The flow graph has problems, so nothing was written:\n")
			printProblems(rep)
		}
		os.Exit(1)
	}
	writeFlowView(docsDir, g)
	if o.json {
		return
	}
	cov := coverage(g)
	pct := 0
	if len(g.Units) > 0 {
		pct = cov * 100 / len(g.Units)
	}
	fmt.Printf("Wrote %s/flowgraph.json and the flow view: %d flows, %d units on %d layers, %d of %d units on a flow (%d%%).\n",
		rel(docsDir), len(g.Flows), len(g.Units), len(g.Layers), cov, len(g.Units), pct)
	if rep.Granularity == "file" {
		fmt.Printf("Units are whole files: the repo has %d functions and methods, more than %d. Pass --granularity symbol to force them.\n", rep.Extracted, maxUnits)
	}
	if missing := rep.FlowPages - len(rep.Blocks); missing > 0 {
		fmt.Printf("%d flow page(s) have no flowgraph block yet, so they aren't in the view.\n", missing)
	}
	if n := len(rep.Unclassified); n > 0 {
		fmt.Printf("%d file(s) matched no layer and were put on Services; add them to %s (see flows --scan).\n", n, rel(layersFile()))
	}
	fmt.Printf("Open %s/flowview.html (works from disk), or the Flow view in the docs viewer.\n", rel(docsDir))
}

// ---------- scan ----------

// Patterns for side effects the agent should tag, matched line by line in each unit's body.
var effectPatterns = []struct {
	effect string
	re     *regexp.Regexp
}{
	// Plain .find( and .delete( are left out: arrays, maps and sets have them too.
	{"db-write", regexp.MustCompile(`\b(?i:INSERT\s+INTO|UPDATE\s+\w+\s+SET|DELETE\s+FROM|UPSERT)\b|\.(createMany|updateMany|upsert|deleteMany|insertOne|insertMany|updateOne|deleteOne|destroy|bulkCreate|persist|flush)\(|\b(prisma|db|knex|repo|repository|em|manager|session)\.(\w+\.)?(create|update|delete|insert|save)\(|->(save|delete|update|insert)\(|::(create|insert|destroy|updateOrCreate)\(|\.Exec(Context)?\(`)},
	{"db-read", regexp.MustCompile(`\b(?i:SELECT\s+.+\s+FROM)\b|\.(findUnique|findFirst|findMany|findOne|findAll|findById|findByPk|aggregate)\(|::(find|where|all|first)\(|->(get|first|find)\(\)|\.Query(Row)?(Context)?\(\s*[^)\s]|\bcursor\.execute\(`)},
	{"external", regexp.MustCompile(`\b(fetch|axios(\.\w+)?|got|ky|superagent|requests\.(get|post|put|patch|delete)|httpx\.\w+|curl_exec|Http::\w+|http\.(Get|Post|NewRequest)|urllib\.request|HttpClient)\s*\(\s*(['"\x60]https?:|[^'"\x60\s)])`)}, // a URL, or a variable; not a same-site path
	{"disk-write", regexp.MustCompile(`\b(fs\.(writeFile|appendFile|createWriteStream|writeFileSync|appendFileSync|rm|unlink|rename|mkdir)\w*|os\.(WriteFile|Create|Remove|RemoveAll|Rename|MkdirAll)|file_put_contents|fwrite|unlink|shutil\.\w+|open\([^)]*['"][wa]b?['"])\s*\(?`)},
	{"queue", regexp.MustCompile(`\b(sendToQueue|publish|sendMessage|enqueue|dispatch|produce|lpush|rpush|xadd)\(|\.add\(\s*['"][\w:-]+['"]\s*,|Queue::push|::dispatch\(`)},
	{"cache", regexp.MustCompile(`\b(redis|cache|memcached?|Cache|Redis)\s*(\.|::|->)\s*(get|set|setex|del|incr|expire|remember|put|forget|hget|hset)\w*\(`)},
}

// Entry points: where flows start.
var entryPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"ui event", regexp.MustCompile(`\b(onSubmit|onClick|on:submit|on:click|@submit|@click|v-on:submit|addEventListener\(\s*['"](submit|click)['"])`)},
	{"scheduled job", regexp.MustCompile(`\b(cron\.schedule|schedule\.every|@Scheduled|@cron|setInterval|->cron\(|->daily\(|->hourly\(|time\.NewTicker|crontab)`)},
	{"queue consumer", regexp.MustCompile(`\b(\.process\(|\.consume\(|@KafkaListener|@RabbitListener|subscribe\(|onMessage|@app\.task|@shared_task|implements ShouldQueue|new Worker\()`)},
	{"cli command", regexp.MustCompile(`\b(\.command\(\s*['"]|@click\.command|argparse\.ArgumentParser|cobra\.Command|protected \$signature|add_parser\()`)},
}

// A switch on command names counts as a CLI only in a file that reads the command line.
var (
	reCaseCommand = regexp.MustCompile(`^\s*case\s+"[a-z][\w-]*"(\s*,\s*"[a-z][\w-]*")*\s*:`)
	reReadsArgs   = regexp.MustCompile(`\b(os\.Args|process\.argv|sys\.argv|\$argv|ARGV)\b`)
	// Lines that define a pattern rather than do the thing (this scanner's own tables, say).
	rePatternDef = regexp.MustCompile(`(regexp\.MustCompile|re\.compile|new RegExp|preg_match)\(`)
)

type scanReport struct {
	Layers       map[string]int      `json:"layers"`
	Units        int                 `json:"units"`
	Granularity  string              `json:"granularity"`
	Unclassified []string            `json:"unclassified"`
	LayersFile   string              `json:"layersFile"`
	Routes       []string            `json:"routes"`
	Entries      []string            `json:"entryPoints"`
	Effects      map[string][]string `json:"sideEffects"` // unit id -> "effect: line text"
	Flows        []scanFlow          `json:"flows"`
	Uncovered    map[string]int      `json:"uncoveredByDistrict"`
	Problems     []Issue             `json:"problems"`
	Stale        []string            `json:"staleFlows"`
	Since        string              `json:"since,omitempty"`
}

type scanFlow struct {
	Page  string `json:"page"`
	Block bool   `json:"block"`
	Steps int    `json:"steps"`
}

func cmdFlowsScan(o options, docsDir string, cfg *Config) {
	g, rep := buildGraph(docsDir, cfg, graphOptions{Granularity: o.granularity})
	s := scanReport{Layers: map[string]int{}, Units: len(g.Units), Granularity: rep.Granularity, Unclassified: rep.Unclassified,
		LayersFile: rel(layersFile()), Effects: map[string][]string{}, Uncovered: map[string]int{}, Problems: rep.Problems}
	for _, u := range g.Units {
		if u.Layer != "" {
			s.Layers[u.Layer]++
		}
	}
	on := map[string]bool{}
	for _, f := range g.Flows {
		for _, st := range f.Steps {
			on[st.Unit] = true
		}
	}
	dname := map[string]string{}
	for _, d := range g.Districts {
		dname[d.ID] = d.ID
	}
	files := map[string]bool{}
	inFile := map[string][]GUnit{}
	for _, u := range g.Units {
		if u.Kind == "code" && len(u.Lines) == 2 {
			inFile[u.File] = append(inFile[u.File], u)
		}
	}
	// smallest is the most specific unit holding a line: a method rather than its class.
	smallest := func(f string, line int) string {
		best, size := "", 1<<30
		for _, u := range inFile[f] {
			if u.Lines[0] <= line && line <= u.Lines[1] && u.Lines[1]-u.Lines[0] < size {
				best, size = u.ID, u.Lines[1]-u.Lines[0]
			}
		}
		return best
	}
	for _, u := range g.Units {
		if u.Kind != "code" {
			continue
		}
		files[u.File] = true
		if !on[u.ID] {
			s.Uncovered[dname[u.District]]++
		}
		if strings.Contains(u.Name, " /") {
			s.Routes = append(s.Routes, fmt.Sprintf("%s  %s:%d", u.Name, u.File, u.Lines[0]))
		}
		// Side effects inside the unit's own lines.
		if len(u.Lines) == 2 && rep.Granularity == "symbol" {
			ls := fileLines(u.File)
			for i := u.Lines[0] - 1; i < u.Lines[1] && i < len(ls); i++ {
				if i == u.Lines[0]-1 || smallest(u.File, i+1) != u.ID { // its own declaration, or a method's line
					continue
				}
				if rePatternDef.MatchString(ls[i]) {
					continue
				}
				for _, p := range effectPatterns {
					if p.re.MatchString(ls[i]) {
						s.Effects[u.ID] = append(s.Effects[u.ID], fmt.Sprintf("%s  line %d: %s", p.effect, i+1, clip(strings.TrimSpace(ls[i]), 100)))
						break
					}
				}
			}
		}
	}
	for _, f := range sortedKeys(files) {
		ls := fileLines(f)
		cli := reReadsArgs.MatchString(strings.Join(ls, "\n"))
		for i, l := range ls {
			if rePatternDef.MatchString(l) {
				continue
			}
			if cli && reCaseCommand.MatchString(l) {
				s.Entries = append(s.Entries, fmt.Sprintf("cli command  %s:%d  %s", f, i+1, clip(strings.TrimSpace(l), 90)))
				continue
			}
			for _, p := range entryPatterns {
				if p.re.MatchString(l) {
					s.Entries = append(s.Entries, fmt.Sprintf("%s  %s:%d  %s", p.kind, f, i+1, clip(strings.TrimSpace(l), 90)))
					break
				}
			}
		}
	}
	blocks := map[string]int{}
	for _, b := range rep.Blocks {
		blocks[b.Page.ID] += len(b.Steps)
	}
	hasBlock := map[string]bool{}
	for _, b := range rep.Blocks {
		hasBlock[b.Page.ID] = true
	}
	for _, p := range flowPages(loadPages(docsDir)) {
		s.Flows = append(s.Flows, scanFlow{Page: p.ID, Block: hasBlock[p.ID], Steps: blocks[p.ID]})
	}
	// Incremental: flows whose units' files changed since the graph on disk was built.
	if prev := readGraphFile(filepath.Join(docsDir, "flowgraph.json")); prev != nil && prev.Repo.Commit != nil && isGit() {
		s.Since = short(*prev.Repo.Commit)
		changed := map[string]bool{}
		if out, ok := git("diff", "--name-only", *prev.Repo.Commit); ok {
			for _, f := range lines(out) {
				changed[f] = true
			}
		}
		if out, ok := git("ls-files", "--others", "--exclude-standard"); ok {
			for _, f := range lines(out) {
				changed[f] = true
			}
		}
		file := map[string]string{}
		for _, u := range g.Units {
			file[u.ID] = u.File
		}
		for _, f := range g.Flows {
			for _, st := range f.Steps {
				if changed[file[st.Unit]] {
					s.Stale = append(s.Stale, f.DocRef)
					break
				}
			}
		}
	}
	if o.json {
		printJSON(s)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Flow scan: %d units (%s granularity)\n\n## Layers\n", s.Units, s.Granularity)
	for _, l := range g.Layers {
		fmt.Fprintf(&b, "- %s (%s): %d units\n", l.ID, l.Name, s.Layers[l.ID])
	}
	if len(s.Unclassified) > 0 {
		fmt.Fprintf(&b, "\n## Files on no layer (put on svc for now; add globs to %s)\n", s.LayersFile)
		for i, f := range s.Unclassified {
			if i == 40 {
				fmt.Fprintf(&b, "- …and %d more\n", len(s.Unclassified)-40)
				break
			}
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	fmt.Fprintf(&b, "\n## Flow pages\n")
	for _, f := range s.Flows {
		state := "no flowgraph block"
		if f.Block {
			state = fmt.Sprintf("%d steps", f.Steps)
		}
		fmt.Fprintf(&b, "- %s: %s\n", f.Page, state)
	}
	if s.Since != "" {
		fmt.Fprintf(&b, "\n## Flows to re-trace (their code changed since %s)\n", s.Since)
		if len(s.Stale) == 0 {
			b.WriteString("- none\n")
		}
		for _, f := range s.Stale {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	section := func(title string, items []string, limit int) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n## %s (%d)\n", title, len(items))
		for i, it := range items {
			if i == limit {
				fmt.Fprintf(&b, "- …and %d more\n", len(items)-limit)
				break
			}
			fmt.Fprintf(&b, "- %s\n", it)
		}
	}
	section("Route handlers (use `route METHOD /path` in a block)", s.Routes, 60)
	section("Other entry points", s.Entries, 60)
	var eff []string
	for _, id := range sortedKeys(s.Effects) {
		for _, e := range s.Effects[id] {
			eff = append(eff, id+"  "+e)
		}
	}
	section("Side-effect candidates (confirm against the code)", eff, 80)
	var unc []string
	dk := sortedKeys(s.Uncovered)
	sort.SliceStable(dk, func(i, j int) bool { return s.Uncovered[dk[i]] > s.Uncovered[dk[j]] })
	for _, d := range dk {
		unc = append(unc, fmt.Sprintf("%s: %d", d, s.Uncovered[d]))
	}
	section("Units on no flow, by district", unc, 25)
	if len(s.Problems) > 0 {
		fmt.Fprintf(&b, "\n## Problems in flowgraph blocks\n")
		for _, i := range s.Problems {
			fmt.Fprintf(&b, "- %s:%d  %s\n", i.Page, i.Line, i.Detail)
		}
	}
	fmt.Print(b.String())
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func readGraphFile(p string) *Graph {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	g := &Graph{}
	if jsonUnmarshal(b, g) != nil {
		return nil
	}
	return g
}
