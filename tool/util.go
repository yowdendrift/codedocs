package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

func die(msg string) {
	fmt.Fprintln(os.Stderr, "codedocs: "+msg)
	os.Exit(2)
}

func posix(p string) string { return filepath.ToSlash(p) }

// rel returns p relative to the repo root, forward slashes.
func rel(p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return posix(p)
	}
	return posix(r)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func readText(p string) (string, error) {
	b, err := os.ReadFile(p)
	return string(b), err
}

func marshal(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		die(err.Error())
	}
	return buf.Bytes()
}

// writeJSON writes atomically: tmp file then rename.
func writeJSON(path string, v any) {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, marshal(v), 0o644); err != nil {
		die(err.Error())
	}
	if err := os.Rename(tmp, path); err != nil {
		die(err.Error())
	}
}

func printJSON(v any) { os.Stdout.Write(marshal(v)) }

// git runs git in the repo root. ok is false if git fails or isn't installed.
func git(args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(out.String()), true
}

var isGitOnce sync.Once
var isGitVal bool

func isGit() bool {
	isGitOnce.Do(func() {
		out, ok := git("rev-parse", "--is-inside-work-tree")
		isGitVal = ok && out == "true"
	})
	return isGitVal
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// semverCmp compares dotted versions numerically; non-numbers count as 0.
func semverCmp(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// ---------- globs ----------

var globCache sync.Map

// globRe turns a repo-relative glob into a regexp. Supports **, *, ?, {a,b}.
func globRe(glob string) *regexp.Regexp {
	if r, ok := globCache.Load(glob); ok {
		return r.(*regexp.Regexp)
	}
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					sb.WriteString("(?:.*/)?")
				} else {
					sb.WriteString(".*")
				}
			} else {
				sb.WriteString("[^/]*")
			}
		case '?':
			sb.WriteString("[^/]")
		case '{':
			end := strings.IndexByte(glob[i:], '}')
			if end < 0 {
				sb.WriteString(`\{`)
				continue
			}
			parts := strings.Split(glob[i+1:i+end], ",")
			for j, p := range parts {
				parts[j] = regexp.QuoteMeta(p)
			}
			sb.WriteString("(?:" + strings.Join(parts, "|") + ")")
			i += end
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if strings.HasSuffix(glob, "/") {
		sb.WriteString(".*")
	}
	sb.WriteString("$")
	r := regexp.MustCompile(sb.String())
	globCache.Store(glob, r)
	return r
}

func matchAny(file string, globs []string) bool {
	for _, g := range globs {
		if globRe(g).MatchString(file) {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
