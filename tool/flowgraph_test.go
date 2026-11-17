package main

// schema/flowgraph.v1.json is the published contract; validateGraph is what the tool runs on
// write. These tests check the fixture passes both, and that both reject the same broken
// graphs, so the two can't drift apart. The JSON Schema check uses a small validator for the
// keywords the schema uses (no third-party modules, see decision 0001).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "schema", "flowgraph.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal("schema isn't valid JSON: ", err)
	}
	return s
}

func loadFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "docs", "design", "flowgraph.sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ---------- a JSON Schema validator for the keywords schema/flowgraph.v1.json uses ----------

type schemaCheck struct{ root map[string]any }

func (c schemaCheck) validate(s map[string]any, v any, at string) []string {
	var errs []string
	fail := func(f string, a ...any) { errs = append(errs, at+": "+fmt.Sprintf(f, a...)) }
	if ref, ok := s["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/$defs/")
		def, ok := c.root["$defs"].(map[string]any)[name].(map[string]any)
		if !ok {
			return []string{at + ": unknown $ref " + ref}
		}
		errs = append(errs, c.validate(def, v, at)...)
	}
	if t, ok := s["type"]; ok {
		types := []string{}
		switch tt := t.(type) {
		case string:
			types = append(types, tt)
		case []any:
			for _, x := range tt {
				types = append(types, x.(string))
			}
		}
		match := false
		for _, ty := range types {
			if jsonType(v, ty) {
				match = true
			}
		}
		if !match {
			fail("want type %v", types)
			return errs
		}
	}
	if cv, ok := s["const"]; ok && fmt.Sprint(cv) != fmt.Sprint(v) {
		fail("want %v", cv)
	}
	if en, ok := s["enum"].([]any); ok {
		found := false
		for _, e := range en {
			if e == v {
				found = true
			}
		}
		if !found {
			fail("%v is not one of %v", v, en)
		}
	}
	if str, ok := v.(string); ok {
		if p, ok := s["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(str) {
			fail("%q doesn't match %s", str, p)
		}
		if n, ok := s["minLength"].(float64); ok && len(str) < int(n) {
			fail("shorter than %v", n)
		}
	}
	if num, ok := v.(float64); ok {
		if n, ok := s["minimum"].(float64); ok && num < n {
			fail("%v is under the minimum %v", num, n)
		}
	}
	if arr, ok := v.([]any); ok {
		if n, ok := s["minItems"].(float64); ok && len(arr) < int(n) {
			fail("fewer than %v items", n)
		}
		if n, ok := s["maxItems"].(float64); ok && len(arr) > int(n) {
			fail("more than %v items", n)
		}
		if items, ok := s["items"].(map[string]any); ok {
			for i, x := range arr {
				errs = append(errs, c.validate(items, x, fmt.Sprintf("%s[%d]", at, i))...)
			}
		}
	}
	if obj, ok := v.(map[string]any); ok {
		props, _ := s["properties"].(map[string]any)
		if req, ok := s["required"].([]any); ok {
			for _, r := range req {
				if _, ok := obj[r.(string)]; !ok {
					fail("missing %q", r)
				}
			}
		}
		for k, x := range obj {
			if ps, ok := props[k].(map[string]any); ok {
				errs = append(errs, c.validate(ps, x, at+"."+k)...)
			} else if ap, ok := s["additionalProperties"].(bool); ok && !ap {
				fail("unexpected property %q", k)
			}
		}
	}
	if any_, ok := s["anyOf"].([]any); ok {
		okOne := false
		for _, sub := range any_ {
			if len(c.validate(sub.(map[string]any), v, at)) == 0 {
				okOne = true
			}
		}
		if !okOne {
			fail("matches none of anyOf")
		}
	}
	if not, ok := s["not"].(map[string]any); ok && len(c.validate(not, v, at)) == 0 {
		fail("matches a schema it must not")
	}
	if cond, ok := s["if"].(map[string]any); ok {
		branch := "else"
		if len(c.validate(cond, v, at)) == 0 {
			branch = "then"
		}
		if sub, ok := s[branch].(map[string]any); ok {
			errs = append(errs, c.validate(sub, v, at)...)
		}
	}
	return errs
}

func jsonType(v any, t string) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := v.(float64)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "null":
		return v == nil
	}
	return false
}

func schemaErrors(t *testing.T, schema map[string]any, b []byte) []string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return schemaCheck{schema}.validate(schema, v, "$")
}

func graphErrors(t *testing.T, b []byte) []string {
	t.Helper()
	var g Graph
	if err := jsonUnmarshal(b, &g); err != nil {
		return []string{err.Error()}
	}
	return validateGraph(&g)
}

// ---------- tests ----------

func TestFixtureIsValid(t *testing.T) {
	schema, fx := loadSchema(t), loadFixture(t)
	if errs := schemaErrors(t, schema, fx); len(errs) > 0 {
		t.Errorf("fixture fails the schema:\n%s", strings.Join(errs, "\n"))
	}
	if errs := graphErrors(t, fx); len(errs) > 0 {
		t.Errorf("fixture fails validateGraph:\n%s", strings.Join(errs, "\n"))
	}
	// The numbers the spec's acceptance criteria quote: 31 of 42 units on a flow.
	var g Graph
	jsonUnmarshal(fx, &g)
	on := map[string]bool{}
	for _, f := range g.Flows {
		for _, s := range f.Steps {
			on[s.Unit] = true
		}
	}
	if len(g.Units) != 42 || len(on) != 31 {
		t.Errorf("fixture coverage = %d of %d, want 31 of 42", len(on), len(g.Units))
	}
}

// Each case breaks the fixture one way. Both validators must reject it (the reference cases
// only validateGraph can catch are marked refOnly), and neither may accept what the other rejects.
func TestSchemaAndValidatorAgree(t *testing.T) {
	schema, fx := loadSchema(t), loadFixture(t)
	cases := []struct {
		name    string
		refOnly bool
		edit    func(g map[string]any)
	}{
		{"wrong version", false, func(g map[string]any) { g["version"] = 2.0 }},
		{"no layers", false, func(g map[string]any) { g["layers"] = []any{} }},
		{"bad unit kind", false, func(g map[string]any) { unit(g, 0)["kind"] = "service" }},
		{"external with a layer", false, func(g map[string]any) { unit(g, len(arr(g, "units"))-1)["layer"] = "svc" }},
		{"code unit without a district", false, func(g map[string]any) { delete(unit(g, 0), "district") }},
		{"bad effect", false, func(g map[string]any) { step(g, 0, 5)["effect"] = "db-delete" }},
		{"negative branch", false, func(g map[string]any) { branch(g, 0, 0)["atStep"] = -1.0 }},
		{"bad colour", false, func(g map[string]any) { flow(g, 0)["color"] = "purple" }},
		{"bad lens", false, func(g map[string]any) { flow(g, 0)["lens"] = "zoomed" }},
		{"flow without steps", false, func(g map[string]any) { flow(g, 0)["steps"] = []any{} }},
		{"lines with one number", false, func(g map[string]any) { unit(g, 0)["lines"] = []any{3.0} }},
		{"step to a missing unit", true, func(g map[string]any) { step(g, 0, 0)["unit"] = "ui.nothing" }},
		{"unit on a missing layer", true, func(g map[string]any) { unit(g, 0)["layer"] = "edge" }},
		{"district on another layer", true, func(g map[string]any) { unit(g, 0)["district"] = "api/middleware" }},
		{"branch past the last step", true, func(g map[string]any) { branch(g, 0, 0)["atStep"] = 99.0 }},
		{"branch to a missing flow", true, func(g map[string]any) { branch(g, 0, 0)["target"] = "refund" }},
		{"duplicate unit", true, func(g map[string]any) { g["units"] = append(arr(g, "units"), arr(g, "units")[0]) }},
	}
	for _, c := range cases {
		var g map[string]any
		json.Unmarshal(fx, &g)
		c.edit(g)
		b, _ := json.Marshal(g)
		se, ge := schemaErrors(t, schema, b), graphErrors(t, b)
		if len(ge) == 0 {
			t.Errorf("%s: validateGraph accepted it", c.name)
		}
		if c.refOnly && len(se) > 0 {
			t.Errorf("%s: expected only validateGraph to catch this, but the schema did too: %v", c.name, se)
		}
		if !c.refOnly && len(se) == 0 {
			t.Errorf("%s: the schema accepted it", c.name)
		}
	}
}

// Everything the schema allows, validateGraph must allow too: every optional field set.
func TestValidatorAcceptsOptionalFields(t *testing.T) {
	schema, fx := loadSchema(t), loadFixture(t)
	var g map[string]any
	json.Unmarshal(fx, &g)
	g["repo"].(map[string]any)["web"] = "https://github.com/o/r/blob/abc/"
	g["repo"].(map[string]any)["root"] = ".."
	u := unit(g, 0)
	u["file"], u["lines"], u["docRef"] = "src/ui/SignupForm.tsx", []any{12.0, 88.0}, "components/auth::signupform"
	f := flow(g, 0)
	f["color"], f["lens"], f["docRef"], f["parent"] = nil, "system", "flows/create-account", "login"
	branch(g, 0, 0)["target"] = "login"
	arr(g, "districts")[0].(map[string]any)["path"] = "src/components/auth"
	b, _ := json.Marshal(g)
	if se := schemaErrors(t, schema, b); len(se) > 0 {
		t.Errorf("schema rejects: %v", se)
	}
	if ge := graphErrors(t, b); len(ge) > 0 {
		t.Errorf("validateGraph rejects: %v", ge)
	}
}

// What the tool writes must pass the schema: marshal a Graph with omitted optionals.
func TestWrittenGraphPassesSchema(t *testing.T) {
	schema := loadSchema(t)
	commit := "abc1234"
	g := &Graph{Version: 1, GeneratedAt: "2026-11-12T20:00:00Z", Repo: GraphRepo{Name: "x", Commit: &commit},
		Layers:    []GLayer{{"ui", "UI", 0}},
		Districts: []GDistrict{{ID: "ui/a", Layer: "ui", Name: "a"}},
		Units:     []GUnit{{ID: "u", Name: "U", Kind: "code", Layer: "ui", District: "ui/a", File: "a.ts", Lines: []int{1, 4}}, {ID: "e", Name: "E", Kind: "external"}},
		Flows:     []GFlow{{ID: "f", Name: "F", Lens: "system", Steps: []GStep{{Unit: "u", Label: "go"}, {Unit: "e", Label: "call", Effect: "external"}}}},
	}
	if ge := validateGraph(g); len(ge) > 0 {
		t.Fatal(ge)
	}
	if se := schemaErrors(t, schema, marshal(g)); len(se) > 0 {
		sort.Strings(se)
		t.Errorf("schema rejects what the tool writes: %v", se)
	}
}

func arr(g map[string]any, k string) []any        { return g[k].([]any) }
func unit(g map[string]any, i int) map[string]any { return arr(g, "units")[i].(map[string]any) }
func flow(g map[string]any, i int) map[string]any { return arr(g, "flows")[i].(map[string]any) }
func step(g map[string]any, f, i int) map[string]any {
	return flow(g, f)["steps"].([]any)[i].(map[string]any)
}
func branch(g map[string]any, f, i int) map[string]any {
	return flow(g, f)["branches"].([]any)[i].(map[string]any)
}

// The example repo's docs, built end to end: blocks, layers.yml, unit extraction, resolution.
func TestExampleGraph(t *testing.T) {
	ex, err := filepath.Abs(filepath.Join("..", "example"))
	if err != nil {
		t.Fatal(err)
	}
	old := root
	root = ex
	defer func() { root = old }()
	docsDir := filepath.Join(ex, "docs")
	g, rep := buildGraph(docsDir, loadConfig(docsDir), graphOptions{})
	if !rep.ok() {
		t.Fatalf("problems: %+v, invalid: %v", rep.Problems, rep.Invalid)
	}
	if se := schemaErrors(t, loadSchema(t), marshal(g)); len(se) > 0 {
		t.Fatalf("schema rejects the example's graph: %v", se)
	}
	var layers []string
	for _, l := range g.Layers {
		layers = append(layers, l.ID)
	}
	if got := strings.Join(layers, ","); got != "ui,svc,data" {
		t.Errorf("layers = %s, want ui,svc,data (from example/.codedocs/layers.yml)", got)
	}
	if len(g.Flows) != 1 || g.Flows[0].ID != "export" {
		t.Fatalf("flows = %+v, want just export", g.Flows)
	}
	f := g.Flows[0]
	if len(f.Steps) != 7 || len(f.Branches) != 2 || f.DocRef != "flows/export" {
		t.Errorf("export flow: %d steps, %d branches, docRef %q; want 7, 2, flows/export", len(f.Steps), len(f.Branches), f.DocRef)
	}
	if f.Steps[3].Effect != "queue" {
		t.Errorf("step 4 effect = %q, want queue", f.Steps[3].Effect)
	}
	byID := map[string]GUnit{}
	for _, u := range g.Units {
		byID[u.ID] = u
	}
	enq := byID[f.Steps[3].Unit]
	if enq.File != "src/render/queue.ts" || enq.Layer != "svc" || len(enq.Lines) != 2 || enq.Lines[0] != 22 {
		t.Errorf("enqueue unit = %+v, want src/render/queue.ts on svc from line 22", enq)
	}
	// Deterministic: a second build gives the same graph, apart from the timestamp.
	g2, _ := buildGraph(docsDir, loadConfig(docsDir), graphOptions{})
	g2.GeneratedAt = g.GeneratedAt
	if string(marshal(g)) != string(marshal(g2)) {
		t.Error("two builds of the same docs differ")
	}
}

// The flow view page must address its script and stylesheet by content hash, or browsers pair
// a new page with a cached old script (/lib/ is cached for a day).
func TestFlowviewPageIsVersioned(t *testing.T) {
	page := string(flowviewPage())
	for _, re := range []string{`src="lib/flowview\.js\?v=[0-9a-f]{8}"`, `href="flowview\.css\?v=[0-9a-f]{8}"`} {
		if !regexp.MustCompile(re).MatchString(page) {
			t.Errorf("flowview.html has no %s; did the tag in tool/assets/flowview.html change?", re)
		}
	}
}
