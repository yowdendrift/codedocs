package main

// The flow graph: the data contract between `codedocs flows` and the glass layers flow view
// (assets/flowview.html). schema/flowgraph.v1.json at the repo root defines it;
// validateGraph enforces the same rules here, plus what JSON Schema can't express (every
// reference resolves). flowgraph_test.go checks the two agree.

import (
	"fmt"
	"regexp"
	"strings"
)

type Graph struct {
	Version     int         `json:"version"`
	GeneratedAt string      `json:"generatedAt"`
	Repo        GraphRepo   `json:"repo"`
	Layers      []GLayer    `json:"layers"`
	Districts   []GDistrict `json:"districts"`
	Units       []GUnit     `json:"units"`
	Flows       []GFlow     `json:"flows"`
}

type GraphRepo struct {
	Name   string  `json:"name"`
	Commit *string `json:"commit"`
	Web    string  `json:"web,omitempty"`
	Root   string  `json:"root,omitempty"`
}

type GLayer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type GDistrict struct {
	ID    string `json:"id"`
	Layer string `json:"layer"`
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`
}

type GUnit struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Layer    string `json:"layer,omitempty"`
	District string `json:"district,omitempty"`
	File     string `json:"file,omitempty"`
	Lines    []int  `json:"lines,omitempty"`
	DocRef   string `json:"docRef,omitempty"`
}

type GFlow struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Lens     string    `json:"lens,omitempty"`
	Color    *string   `json:"color"`
	DocRef   string    `json:"docRef,omitempty"`
	Parent   string    `json:"parent,omitempty"`
	Steps    []GStep   `json:"steps"`
	Branches []GBranch `json:"branches,omitempty"`
}

type GStep struct {
	Unit   string `json:"unit"`
	Label  string `json:"label"`
	Effect string `json:"effect,omitempty"`
}

type GBranch struct {
	AtStep int    `json:"atStep"`
	Label  string `json:"label"`
	Target string `json:"target,omitempty"`
}

var (
	unitKinds   = set("code", "store", "file", "external", "queue", "cache")
	stepEffects = set("db-read", "db-write", "external", "disk-write", "queue", "cache")
	flowLenses  = set("system", "ui")
	reGraphID   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/$-]*$`)
	reHexColor  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

// validateGraph returns every problem with g, worded for a person; empty means valid.
func validateGraph(g *Graph) []string {
	var errs []string
	bad := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	id := func(what, v string) {
		if !reGraphID.MatchString(v) {
			bad("%s: bad id %q (letters, digits and _ . : / $ - only)", what, v)
		}
	}
	if g.Version != 1 {
		bad("version must be 1, not %d", g.Version)
	}
	if g.GeneratedAt == "" {
		bad("generatedAt is missing")
	}
	if g.Repo.Name == "" {
		bad("repo.name is missing")
	}
	if len(g.Layers) == 0 {
		bad("there are no layers")
	}
	layers := map[string]bool{}
	for i, l := range g.Layers {
		id(fmt.Sprintf("layers[%d]", i), l.ID)
		if layers[l.ID] {
			bad("layer %q is defined twice", l.ID)
		}
		layers[l.ID] = true
		if l.Name == "" {
			bad("layer %q has no name", l.ID)
		}
	}
	districts := map[string]string{} // id -> layer
	for i, d := range g.Districts {
		id(fmt.Sprintf("districts[%d]", i), d.ID)
		if _, dup := districts[d.ID]; dup {
			bad("district %q is defined twice", d.ID)
		}
		districts[d.ID] = d.Layer
		if !layers[d.Layer] {
			bad("district %q is on layer %q, which doesn't exist", d.ID, d.Layer)
		}
		if d.Name == "" {
			bad("district %q has no name", d.ID)
		}
	}
	units := map[string]bool{}
	for i, u := range g.Units {
		id(fmt.Sprintf("units[%d]", i), u.ID)
		if units[u.ID] {
			bad("unit %q is defined twice", u.ID)
		}
		units[u.ID] = true
		if u.Name == "" {
			bad("unit %q has no name", u.ID)
		}
		if !unitKinds[u.Kind] {
			bad("unit %q has kind %q; it must be one of code, store, file, external, queue, cache", u.ID, u.Kind)
		}
		if u.Kind == "external" {
			if u.Layer != "" || u.District != "" {
				bad("unit %q is external, so it can't have a layer or district", u.ID)
			}
		} else {
			if u.Layer == "" || u.District == "" {
				bad("unit %q needs a layer and a district", u.ID)
			} else if !layers[u.Layer] {
				bad("unit %q is on layer %q, which doesn't exist", u.ID, u.Layer)
			} else if dl, ok := districts[u.District]; !ok {
				bad("unit %q is in district %q, which doesn't exist", u.ID, u.District)
			} else if dl != u.Layer {
				bad("unit %q is on layer %q but its district %q is on layer %q", u.ID, u.Layer, u.District, dl)
			}
		}
		if u.Lines != nil && (len(u.Lines) != 2 || u.Lines[0] < 1 || u.Lines[1] < 1) {
			bad("unit %q: lines must be [first, last], both 1 or more", u.ID)
		}
	}
	flows := map[string]bool{}
	for _, f := range g.Flows {
		flows[f.ID] = true
	}
	seen := map[string]bool{}
	for i, f := range g.Flows {
		id(fmt.Sprintf("flows[%d]", i), f.ID)
		if seen[f.ID] {
			bad("flow %q is defined twice", f.ID)
		}
		seen[f.ID] = true
		if f.Name == "" {
			bad("flow %q has no name", f.ID)
		}
		if f.Lens != "" && !flowLenses[f.Lens] {
			bad("flow %q has lens %q; it must be system or ui", f.ID, f.Lens)
		}
		if f.Color != nil && !reHexColor.MatchString(*f.Color) {
			bad("flow %q has color %q; use #rrggbb or null", f.ID, *f.Color)
		}
		if f.Parent != "" && !flows[f.Parent] {
			bad("flow %q has parent %q, which isn't a flow", f.ID, f.Parent)
		}
		if len(f.Steps) == 0 {
			bad("flow %q has no steps", f.ID)
		}
		for j, s := range f.Steps {
			if !units[s.Unit] {
				bad("flow %q step %d names unit %q, which doesn't exist", f.ID, j+1, s.Unit)
			}
			if strings.TrimSpace(s.Label) == "" {
				bad("flow %q step %d has no label", f.ID, j+1)
			}
			if s.Effect != "" && !stepEffects[s.Effect] {
				bad("flow %q step %d has effect %q; it must be one of db-read, db-write, external, disk-write, queue, cache", f.ID, j+1, s.Effect)
			}
		}
		for _, b := range f.Branches {
			if b.AtStep < 0 || b.AtStep >= len(f.Steps) {
				bad("flow %q has a branch %q at step index %d, but its steps are numbered 0 to %d", f.ID, b.Label, b.AtStep, len(f.Steps)-1)
			}
			if b.Label == "" {
				bad("flow %q has a branch with no label", f.ID)
			}
			if b.Target != "" && !flows[b.Target] {
				bad("flow %q has a branch to %q, which isn't a flow", f.ID, b.Target)
			}
		}
	}
	return errs
}
