package main

// The viewer carries JavaScript copies of two pieces of Go logic so static exports work
// without a server: glob matching (globRe) and go-to-definition ranking (rankDefinitions).
// This test runs the viewer's copies in Node, on the same cases as the Go originals, and
// fails if any answer differs. The copies live in assets/index.html between
// "// parity:begin" and "// parity:end".
//
// Needs Node. Without it the test is skipped locally, and fails in CI (CI is set there).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var parityGlobs = []string{
	"tool/**", "tool/*.go", "**/*.md", "**/README.md", "src/{a,b}/*.ts", "docs/", "a?c.txt",
	"x.y+z(1).go", "{unterminated", "skills/codedocs/bin/**", "*", "**", ".github/**",
	"tool/assets/lib/*.js.gz", "a/**/b.go", "**/", "src/**/*.{ts,tsx}", "[abc].go", "$weird^.md",
}

var parityPaths = []string{
	"tool/main.go", "tool/assets/index.html", "README.md", "docs/README.md", "docs/a/b.md",
	"src/a/x.ts", "src/c/x.ts", "src/a/deep/y.tsx", "abc.txt", "a/c.txt", "x.y+z(1).go",
	"xay+z(1).go", "{unterminated", "skills/codedocs/bin/codedocs", "a/b.go", "a/x/y/b.go",
	".github/workflows/ci.yml", "tool/assets/lib/d3.js.gz", "toolbox/x.go", "a.go", "[abc].go",
	"$weird^.md",
}

type defCase struct {
	Name    string                    `json:"name"`
	All     []Def                     `json:"all"`
	From    string                    `json:"from"`
	Imports map[string]resolvedImport `json:"imports"`
	Links   []Link                    `json:"links"`
}

func parityDefCases() []defCase {
	many := []Def{}
	for i := 0; i < 55; i++ {
		many = append(many, Def{File: fmt.Sprintf("pkg%02d/f.go", 54-i), Line: i + 1})
	}
	spread := []Def{
		{File: "z/far.go", Line: 3}, {File: "src/ui/b.ts", Line: 9}, {File: "src/render/q.ts", Line: 4},
		{File: "src/render/q.ts", Line: 2}, {File: "a/x.ts", Line: 1}, {File: "root.go", Line: 7},
		{File: "src/ui/a.ts", Line: 5},
	}
	return []defCase{
		{Name: "Thing", All: spread, From: "src/ui/a.ts",
			Imports: map[string]resolvedImport{"Thing": {Path: "src/render/q.ts", Line: 12}}},
		{Name: "Run", All: spread, From: "cmd/main.go",
			Imports: map[string]resolvedImport{"Run": {Path: "src/render", Dir: true}}},
		{Name: "Run", All: spread, From: "cmd/main.go",
			Imports: map[string]resolvedImport{"Run": {Path: "nowhere", Dir: true}}},
		{Name: "Local", All: spread, From: "src/render/q.ts"},
		{Name: "Near", All: spread, From: "src/ui/c.ts",
			Links: []Link{{Line: 1, Text: "../render/q", Path: "src/render/q.ts"}, {Line: 2, Text: "a", Path: "a", Dir: true}}},
		{Name: "NoFrom", All: spread, From: ""},
		{Name: "Root", All: spread, From: "other.go"},
		{Name: "Capped", All: many, From: "pkg03/g.go"},
		{Name: "Missing", All: []Def{}, From: "src/ui/a.ts"},
		{Name: "Leading", All: []Def{{File: "/abs/x.go", Line: 1}, {File: "x.go", Line: 2}}, From: "/abs/y.go"},
	}
}

func parityBlock(t *testing.T) string {
	b, err := os.ReadFile(filepath.Join("assets", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "// parity:begin"), strings.Index(s, "// parity:end")
	if i < 0 || j < i {
		t.Fatal("assets/index.html: parity markers not found")
	}
	return s[i:j]
}

func TestViewerParity(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("node is required for the viewer parity test in CI")
		}
		t.Skip("node not installed; skipping the viewer parity test")
	}

	// Go's answers.
	var globPairs [][2]string
	var goGlobs []bool
	for _, g := range parityGlobs {
		for _, p := range parityPaths {
			globPairs = append(globPairs, [2]string{g, p})
			goGlobs = append(goGlobs, globRe(g).MatchString(p))
		}
	}
	cases := parityDefCases()
	goDefs := make([][]Def, len(cases))
	for i, c := range cases {
		goDefs[i] = rankDefinitions(c.Name, c.All, c.From, c.Imports, c.Links)
	}

	// The viewer's answers, from the same inputs.
	harness := parityBlock(t) + `
const input = JSON.parse(require('fs').readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify({
  globs: input.globs.map(([g, p]) => globRe(g).test(p)),
  defs: input.defs.map(c => rankDefinitions(c.name, c.all, c.from, c.imports, c.links)),
}));
`
	script := filepath.Join(t.TempDir(), "parity.js")
	if err := os.WriteFile(script, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]any{"globs": globPairs, "defs": cases})
	cmd := exec.Command(node, script)
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var js struct {
		Globs []bool            `json:"globs"`
		Defs  []json.RawMessage `json:"defs"`
	}
	if err := json.Unmarshal(out, &js); err != nil {
		t.Fatalf("bad output from node: %v\n%s", err, out)
	}

	if len(js.Globs) != len(goGlobs) {
		t.Fatalf("got %d glob answers from the viewer, want %d", len(js.Globs), len(goGlobs))
	}
	for i, want := range goGlobs {
		if js.Globs[i] != want {
			t.Errorf("glob %q on %q: Go says %v, viewer says %v", globPairs[i][0], globPairs[i][1], want, js.Globs[i])
		}
	}
	for i, c := range cases {
		var jsDefs []Def
		if err := json.Unmarshal(js.Defs[i], &jsDefs); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		g, _ := json.Marshal(nonNil(goDefs[i]))
		v, _ := json.Marshal(nonNil(jsDefs))
		if !bytes.Equal(g, v) {
			t.Errorf("rankDefinitions(%q from %q):\n  Go:     %s\n  viewer: %s", c.Name, c.From, g, v)
		}
	}
}

func nonNil(d []Def) []Def {
	if d == nil {
		return []Def{}
	}
	return d
}
