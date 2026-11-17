package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A calls B, B calls C (in another file it imports), D is called by nothing; a C in a file A's
// file can't see isn't matched; a method's calls belong to the method, not its class.
func TestReach(t *testing.T) {
	dir := t.TempDir()
	old := root
	root = dir
	defer func() { root = old }()
	files := map[string]string{
		"reachfx/a.js":     "export function A() {\n  B();\n}\nexport function B() {\n  return C(1);\n}\n",
		"reachfx/c.js":     "export function C(x) {\n  return x;\n}\nexport function D() {\n  return 2;\n}\n",
		"reachfx/other.js": "export function E() {\n  return 3;\n}\n",
		"reachfx/k.js":     "class K {\n  m() {\n    E();\n  }\n}\n",
	}
	for f, s := range files {
		os.MkdirAll(filepath.Join(dir, "reachfx"), 0o755)
		os.WriteFile(filepath.Join(dir, f), []byte(s), 0o644)
	}
	units := []reachUnit{
		{ID: "A", File: "reachfx/a.js", Name: "A", Lines: [2]int{1, 3}},
		{ID: "B", File: "reachfx/a.js", Name: "B", Lines: [2]int{4, 6}},
		{ID: "C", File: "reachfx/c.js", Name: "C", Lines: [2]int{1, 3}},
		{ID: "D", File: "reachfx/c.js", Name: "D", Lines: [2]int{4, 6}},
		{ID: "E", File: "reachfx/other.js", Name: "E", Lines: [2]int{1, 3}},
		{ID: "K", File: "reachfx/k.js", Name: "K", Lines: [2]int{1, 5}},
		{ID: "K.m", File: "reachfx/k.js", Name: "m", Lines: [2]int{2, 4}},
	}
	sees := func(from, to string) bool {
		return from == to || from == "reachfx/a.js" && to == "reachfx/c.js" || from == "reachfx/k.js" && to == "reachfx/other.js"
	}
	edges := callEdges(units, sees)
	if got := edges["K"]; len(got) != 0 {
		t.Errorf("the class K has its method's calls: %v", got)
	}
	if got := edges["K.m"]; len(got) != 1 || got[0] != "E" {
		t.Errorf("K.m calls %v, want [E]", got)
	}
	flows := []GFlow{{ID: "f", Steps: []GStep{{Unit: "A"}}}}
	r := reachFrom(flows, map[string]bool{"A": true}, nil, edges)
	if r["B"] != (GReach{Flow: "f", Via: "A"}) || r["C"] != (GReach{Flow: "f", Via: "B"}) {
		t.Errorf("reach = %v, want B via A and C via B", r)
	}
	for _, id := range []string{"A", "D", "E"} {
		if _, ok := r[id]; ok {
			t.Errorf("%s shouldn't be reached", id)
		}
	}
	// An entry handler stops the walk.
	if r := reachFrom(flows, map[string]bool{"A": true}, map[string]bool{"B": true}, edges); len(r) != 0 {
		t.Errorf("reach went through the entry handler B: %v", r)
	}
	// C in a file A can't see: not matched.
	edges = callEdges(units, func(from, to string) bool { return from == to })
	if got := edges["B"]; len(got) != 0 {
		t.Errorf("B calls %v across files it can't see", got)
	}
}
