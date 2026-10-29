package main

// `codedocs find <term...>`: where a subject lives in the repo, cheaply. Used by
// /codedocs add to scope a deep dive before reading any code.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func cmdFind(o options) {
	if len(o.pos) == 0 {
		die("usage: codedocs find <term> [term…]   (case-insensitive; matches paths, contents and definitions)")
	}
	terms := make([]string, len(o.pos))
	for i, t := range o.pos {
		terms[i] = strings.ToLower(t)
	}
	docsDir := findDocsDir(o, false)
	var ignore []string
	if docsDir != "" {
		ignore = append(ignore, rel(docsDir)+"/**")
		ignore = append(ignore, loadConfig(docsDir).Ignore...)
	}
	files := sourceFiles(append(ignore, secretGlobs...))
	type hit struct {
		file     string
		pathHit  bool
		count    int
		lines    int
		termsHit map[string]bool
	}
	var hits []*hit
	for _, f := range files {
		h := &hit{file: f, termsHit: map[string]bool{}}
		lf := strings.ToLower(f)
		for _, t := range terms {
			if strings.Contains(lf, t) {
				h.pathHit = true
				h.termsHit[t] = true
			}
		}
		full := filepath.Join(root, filepath.FromSlash(f))
		if st, err := os.Stat(full); err == nil && st.Size() < 1<<20 {
			if b, err := os.ReadFile(full); err == nil {
				lc := strings.ToLower(string(b))
				h.lines = strings.Count(lc, "\n") + 1
				for _, t := range terms {
					if n := strings.Count(lc, t); n > 0 {
						h.count += n
						h.termsHit[t] = true
					}
				}
			}
		}
		if h.pathHit || h.count > 0 {
			hits = append(hits, h)
		}
	}
	// Definitions whose names contain a term.
	idx := defs.get(files)
	var defHits []string
	for _, name := range sortedKeys(idx) {
		ln := strings.ToLower(name)
		for _, t := range terms {
			if strings.Contains(ln, t) {
				for _, d := range idx[name] {
					defHits = append(defHits, fmt.Sprintf("%s  %s:%d", name, d.File, d.Line))
				}
				break
			}
		}
	}
	score := func(h *hit) int {
		s := h.count + 3*len(h.termsHit)
		if h.pathHit {
			s += 25
		}
		return s
	}
	sort.SliceStable(hits, func(i, j int) bool { return score(hits[i]) > score(hits[j]) })
	if o.json {
		out := []map[string]any{}
		for _, h := range hits {
			out = append(out, map[string]any{"file": h.file, "path_match": h.pathHit, "mentions": h.count, "lines": h.lines})
		}
		printJSON(map[string]any{"terms": terms, "files": out, "definitions": defHits})
		return
	}
	// Folders, by total mentions.
	type dir struct {
		files, mentions int
	}
	dirs := map[string]*dir{}
	for _, h := range hits {
		d := path.Dir(h.file)
		if dirs[d] == nil {
			dirs[d] = &dir{}
		}
		dirs[d].files++
		dirs[d].mentions += h.count
	}
	dk := sortedKeys(dirs)
	sort.SliceStable(dk, func(i, j int) bool { return dirs[dk[i]].mentions > dirs[dk[j]].mentions })
	fmt.Printf("%q: %d file(s) of %d\n\nFolders (by mentions):\n", strings.Join(o.pos, " "), len(hits), len(files))
	for i, d := range dk {
		if i == 20 {
			fmt.Printf("  … %d more folders\n", len(dk)-20)
			break
		}
		fmt.Printf("  %-60s %3d files, %4d mentions\n", d+"/", dirs[d].files, dirs[d].mentions)
	}
	fmt.Println("\nTop files:")
	for i, h := range hits {
		if i == 30 {
			fmt.Printf("  … %d more files\n", len(hits)-30)
			break
		}
		mark := ""
		if h.pathHit {
			mark = " [name]"
		}
		fmt.Printf("  %-70s %4d mentions, %5d lines%s\n", h.file, h.count, h.lines, mark)
	}
	if len(defHits) > 0 {
		fmt.Println("\nDefinitions with a matching name:")
		for i, d := range defHits {
			if i == 40 {
				fmt.Printf("  … %d more\n", len(defHits)-40)
				break
			}
			fmt.Println("  " + d)
		}
	}
}
