package main

// export: a static copy of the docs that works on any web host (GitHub Pages, S3, a
// plain web server), code panel included. Nothing in it needs the codedocs binary:
//
//   index.html, viewer.css, theme.css, lib/   the viewer and its libraries
//   manifest.json, content/                   the docs (machine-specific paths removed)
//   code/index.json                           where the source lives, and the file list
//   code/defs.json                            name -> definitions, for Ctrl+click (loaded on demand)
//   code/meta/<path>.json                     per file: its import links and where imported names live
//
// The source itself isn't copied. The viewer fetches each file from code/index.json's
// "source" URL, normally raw.githubusercontent.com pinned to the commit the docs describe,
// so line numbers in the docs always match the code shown.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type codeIndex struct {
	Version string   `json:"version"`
	Project string   `json:"project"`
	Source  string   `json:"source"`         // base URL: source + path = raw file
	Web     string   `json:"web,omitempty"`  // base URL: web + path = the file on the forge's website
	Ref     string   `json:"ref,omitempty"`  // commit the source URL is pinned to
	Repo    string   `json:"repo,omitempty"` // owner/repo, for display
	Files   []string `json:"files"`
	Meta    []int    `json:"meta,omitempty"` // indexes of files that have code/meta/<path>.json
}

type codeMeta struct {
	Links   []Link                    `json:"links,omitempty"`
	Imports map[string]resolvedImport `json:"imports,omitempty"` // local name -> where it's defined
	Syms    map[string]int            `json:"syms,omitempty"`    // "path#Symbol" -> line, for link targets
}

var reGitHubRemote = regexp.MustCompile(`github\.com[:/]+([^/\s]+)/([^/\s]+?)(?:\.git)?/?$`)

func cmdExport(o options) {
	if len(o.pos) == 0 {
		die("usage: codedocs export OUTPUT_FOLDER [--github OWNER/REPO] [--ref COMMIT] [--source-url URL --web-url URL]")
	}
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	out, _ := filepath.Abs(o.pos[0])
	if out == root || strings.HasPrefix(docsDir+string(filepath.Separator), out+string(filepath.Separator)) {
		die("the output folder can't be the repo or contain the docs folder: " + rel(out))
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 && !fileExists(filepath.Join(out, "code", "index.json")) && !fileExists(filepath.Join(out, "manifest.json")) {
		die(rel(out) + " exists and doesn't look like an earlier export; pick an empty folder")
	}
	os.RemoveAll(out)
	write := func(name string, b []byte) {
		p := filepath.Join(out, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			die(err.Error())
		}
	}

	// The viewer, its libraries and the theme.
	write("index.html", readAsset("index.html"))
	write("viewer.css", readAsset("viewer.css"))
	entries, _ := assets.ReadDir("assets/lib")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			write("lib/"+strings.TrimSuffix(e.Name(), ".gz"), gunzip(readAsset("lib/"+e.Name())))
		}
	}
	if b, err := os.ReadFile(filepath.Join(docsDir, "theme.css")); err == nil {
		write("theme.css", b)
	} else {
		b, _ := defaultTheme()
		write("theme.css", b)
	}

	// The docs: content as-is, manifest without paths from this machine.
	contentDir := filepath.Join(docsDir, "content")
	filepath.WalkDir(contentDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err == nil {
			r, _ := filepath.Rel(docsDir, p)
			write(filepath.ToSlash(r), b)
		}
		return nil
	})
	m := buildManifest(docsDir, cfg)
	m.RepoRootAbs = ""
	m.Editor = "none"
	m.Static = true
	write("manifest.json", marshal(m))
	fmt.Printf("Exported %d pages to %s\n", len(m.Pages), rel(out))

	// Where the source lives.
	src, web, ref, repo := exportSource(o, cfg)
	if src == "" {
		fmt.Println("No code panel: this isn't a GitHub repo (no github.com remote). Pass --github OWNER/REPO, or --source-url and --web-url, to add one.")
		return
	}
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if web != "" && !strings.HasSuffix(web, "/") {
		web += "/"
	}

	// The files the code panel can show, and what it needs to know about each.
	ctx := buildRepoCtx(sourceFiles(secretGlobs), cfg)
	docsRel := posix(rel(docsDir)) + "/"
	var files []string
	texts := map[string]string{}
	for _, f := range ctx.files {
		if strings.HasPrefix(f, docsRel) {
			continue // the docs themselves are in the export already
		}
		full := filepath.Join(root, filepath.FromSlash(f))
		st, err := os.Stat(full)
		if err != nil || st.Size() > maxCodeFile {
			continue
		}
		b, err := os.ReadFile(full)
		if err != nil || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
			continue
		}
		files = append(files, f)
		texts[f] = strings.ReplaceAll(string(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))), "\r\n", "\n")
	}
	sort.Strings(files)

	// Definitions by name, for Ctrl+click. Stored as name -> [[file index, line], …] to keep it small.
	idx := map[string]int{}
	for i, f := range files {
		idx[f] = i
	}
	compact := map[string][][2]int{}
	for name, ds := range defs.get(files) {
		for _, d := range ds {
			if i, ok := idx[d.File]; ok {
				compact[name] = append(compact[name], [2]int{i, d.Line})
			}
		}
	}
	write("code/defs.json", compactJSON(compact))

	var metas []int
	for i, f := range files {
		fl := linksFor(ctx, f, texts[f])
		cm := codeMeta{Links: fl.Links, Syms: map[string]int{}}
		for _, l := range fl.Links {
			if l.Symbol != "" && !l.Dir {
				if n := resolveSymbol(l.Path, l.Symbol); n > 0 {
					cm.Syms[l.Path+"#"+l.Symbol] = n
				}
			}
		}
		cm.Imports = resolveImports(ctx, fl)
		if len(cm.Syms) == 0 {
			cm.Syms = nil
		}
		if cm.Links == nil && cm.Imports == nil {
			continue
		}
		write("code/meta/"+f+".json", compactJSON(cm))
		metas = append(metas, i)
	}
	write("code/index.json", compactJSON(codeIndex{Version: version, Project: cfg.Project, Source: src, Web: web, Ref: ref, Repo: repo, Files: files, Meta: metas}))
	fmt.Printf("Code panel: %d files from %s (%d with import links)\n", len(files), strings.TrimSuffix(src, "/"), len(metas))

	// The source URL is pinned to a commit: warn if that commit isn't on the remote yet.
	if ref != "" && isGit() {
		if outp, ok := git("branch", "-r", "--contains", ref); ok && strings.TrimSpace(outp) == "" {
			fmt.Printf("Warning: commit %s isn't on any remote branch yet. Push it, or the code panel will show \"not found\".\n", short(ref))
		}
	}
	if m.LastSynced != nil && ref != "" && !strings.HasPrefix(ref, *m.LastSynced) && !strings.HasPrefix(*m.LastSynced, ref) {
		fmt.Printf("Note: the docs describe commit %s but the code panel shows %s; run /codedocs update, or pass --ref %s.\n", short(*m.LastSynced), short(ref), short(*m.LastSynced))
	}
}

func compactJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		die(err.Error())
	}
	return b
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// exportSource works out where the exported viewer fetches source files from.
func exportSource(o options, cfg *Config) (src, web, ref, repo string) {
	if o.sourceURL != "" {
		return o.sourceURL, o.webURL, o.ref, ""
	}
	repo = o.github
	if repo == "" && isGit() {
		if url, ok := git("remote", "get-url", "origin"); ok {
			if m := reGitHubRemote.FindStringSubmatch(strings.TrimSpace(url)); m != nil {
				repo = m[1] + "/" + m[2]
			}
		}
	}
	if repo == "" {
		return "", "", "", ""
	}
	ref = o.ref
	if ref == "" && cfg.LastSynced != nil && *cfg.LastSynced != "" {
		ref = *cfg.LastSynced // the commit the docs describe
	}
	if ref == "" && isGit() {
		if h, ok := git("rev-parse", "HEAD"); ok {
			ref = strings.TrimSpace(h)
		}
	}
	if ref == "" {
		ref = "HEAD"
	} else if isGit() {
		if full, ok := git("rev-parse", ref+"^{commit}"); ok {
			ref = strings.TrimSpace(full)
		}
	}
	// The repo root may be a subfolder of the git repository.
	prefix := ""
	if isGit() {
		if p, ok := git("rev-parse", "--show-prefix"); ok {
			prefix = strings.TrimSpace(p)
		}
	}
	src = "https://raw.githubusercontent.com/" + repo + "/" + ref + "/" + prefix
	web = "https://github.com/" + repo + "/blob/" + ref + "/" + prefix
	return src, web, ref, repo
}
