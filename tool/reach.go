package main

// Reach: the code a documented flow runs without naming it. A flow names the hops that matter,
// not every helper they call, so "on no flow" overstates what's undocumented. computeReach
// follows calls out of every step's unit and marks each code unit it reaches with the first
// flow (in flow order) and the unit that called it. Like findCallers it's approximate: a call
// is a name followed by "(", matched to units of that name in the files the caller can see
// (its own file, the rest of its Go package, the files it imports). A function only passed as
// a value (an event handler, a callback) isn't followed.

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// reachUnit is what computeReach needs to know about a code unit.
type reachUnit struct {
	ID    string
	File  string
	Name  string // the bare name calls use: the last part of Class.method
	Lines [2]int
}

var reCallTok = regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*(?:<[^<>()]*>)?\(`)

// callEdges returns, for each unit, the units it calls (deduplicated, in source order).
// sees(from, to) says whether file from can see file to.
func callEdges(units []reachUnit, sees func(from, to string) bool) map[string][]string {
	byName := map[string][]int{}
	inFile := map[string][]int{}
	for i, u := range units {
		if u.Name != "" {
			byName[u.Name] = append(byName[u.Name], i)
		}
		inFile[u.File] = append(inFile[u.File], i)
	}
	// owner is the smallest unit holding a line, so a class's lines that belong to its methods
	// count as the method's calls, not the class's.
	owner := func(f string, line int) int {
		best, size := -1, 1<<30
		for _, i := range inFile[f] {
			u := units[i]
			if u.Lines[0] <= line && line <= u.Lines[1] && u.Lines[1]-u.Lines[0] < size {
				best, size = i, u.Lines[1]-u.Lines[0]
			}
		}
		return best
	}
	edges := map[string][]string{}
	for i, u := range units {
		if u.Lines[0] < 1 {
			continue
		}
		ls := fileLines(u.File)
		seen := map[string]bool{}
		for ln := u.Lines[0]; ln <= u.Lines[1] && ln <= len(ls); ln++ {
			text := ls[ln-1]
			if len(text) > 400 || reCallNoise.MatchString(text) || owner(u.File, ln) != i {
				continue
			}
			for _, m := range reCallTok.FindAllStringSubmatchIndex(text, -1) {
				if m[2] > 0 { // part of a longer name (the regexp can start mid-word)
					if c := text[m[2]-1]; c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
						continue
					}
				}
				name := text[m[2]:m[3]]
				if ln == u.Lines[0] && name == u.Name {
					continue // its own declaration
				}
				for _, j := range byName[name] {
					t := units[j]
					if j == i || seen[t.ID] || !sees(u.File, t.File) {
						continue
					}
					seen[t.ID] = true
					edges[u.ID] = append(edges[u.ID], t.ID)
				}
			}
		}
	}
	return edges
}

// reachFrom walks the call edges out of each flow's steps, flows in order, breadth first. A
// unit on a flow is never marked (it's documented) and isn't walked through either: its own
// flow covers what it calls. Nor is an entry handler (stop): a CLI command's function or what
// an event listener calls starts a flow of its own, even when a documented step (main, a
// page's init) is what dispatches to it. It returns unit id -> {flow, via}.
func reachFrom(flows []GFlow, onFlow, stop map[string]bool, edges map[string][]string) map[string]GReach {
	out := map[string]GReach{}
	for _, f := range flows {
		var queue []string
		seen := map[string]bool{}
		for _, s := range f.Steps {
			if !seen[s.Unit] {
				seen[s.Unit] = true
				queue = append(queue, s.Unit)
			}
		}
		for len(queue) > 0 {
			from := queue[0]
			queue = queue[1:]
			for _, to := range edges[from] {
				if seen[to] || onFlow[to] || stop[to] {
					continue
				}
				seen[to] = true
				if _, done := out[to]; done {
					continue // an earlier flow reached it and walked on from it
				}
				out[to] = GReach{Flow: f.ID, Via: from}
				queue = append(queue, to)
			}
		}
	}
	return out
}

// sourceVisibility says whether one file can call into another: the same file, the same Go
// package, or a file it imports (by the code panel's import resolution).
func sourceVisibility(c *repoCtx) func(from, to string) bool {
	by := importers.get(c) // to -> files importing it (for Go: folder -> files importing it)
	imports := map[string]map[string]bool{}
	for to, list := range by {
		for _, from := range list {
			if imports[from] == nil {
				imports[from] = map[string]bool{}
			}
			imports[from][to] = true
		}
	}
	return func(from, to string) bool {
		if from == to {
			return true
		}
		if extOf(from) == "go" && extOf(to) == "go" && path.Dir(from) == path.Dir(to) {
			return true
		}
		return imports[from][to] || imports[from][path.Dir(to)]
	}
}

// computeReach sets Reached on the code units of g that a flow runs without naming.
func computeReach(g *Graph, names map[string]string, sees func(from, to string) bool) {
	onFlow := map[string]bool{}
	for _, f := range g.Flows {
		for _, s := range f.Steps {
			onFlow[s.Unit] = true
		}
	}
	var units []reachUnit
	for _, u := range g.Units {
		if u.Kind != "code" || len(u.Lines) != 2 || u.File == "" {
			continue
		}
		units = append(units, reachUnit{ID: u.ID, File: u.File, Name: names[u.ID], Lines: [2]int{u.Lines[0], u.Lines[1]}})
	}
	sort.SliceStable(units, func(i, j int) bool { return units[i].File < units[j].File })
	reached := reachFrom(g.Flows, onFlow, entryTargets(units, sees), callEdges(units, sees))
	for i := range g.Units {
		if r, ok := reached[g.Units[i].ID]; ok {
			r := r
			g.Units[i].Reached = &r
		}
	}
}

// entryTargets: the units an entry point hands off to. The calls on a CLI dispatcher's
// `case "name":` line and the next, and on an event listener's line and the four after it
// (the same spans flows --scan checks).
func entryTargets(units []reachUnit, sees func(from, to string) bool) map[string]bool {
	byName := map[string][]reachUnit{}
	files := map[string]bool{}
	for _, u := range units {
		if u.Name != "" {
			byName[u.Name] = append(byName[u.Name], u)
		}
		files[u.File] = true
	}
	out := map[string]bool{}
	mark := func(f string, ls []string, i, span int) {
		for k := i; k < len(ls) && k <= i+span; k++ {
			for _, m := range reCallTok.FindAllStringSubmatch(ls[k], -1) {
				for _, u := range byName[m[1]] {
					if sees(f, u.File) {
						out[u.ID] = true
					}
				}
			}
		}
	}
	for f := range files {
		ls := fileLines(f)
		cli := reReadsArgs.MatchString(strings.Join(ls, "\n"))
		for i, l := range ls {
			if rePatternDef.MatchString(l) {
				continue
			}
			if cli && reCaseCommand.MatchString(l) {
				mark(f, ls, i, 1)
				continue
			}
			for _, p := range entryPatterns {
				if p.re.MatchString(l) {
					mark(f, ls, i, 4)
					break
				}
			}
		}
	}
	return out
}
