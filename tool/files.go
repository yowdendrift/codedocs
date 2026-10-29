package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var defaultIgnore = []string{
	".git/**", "node_modules/**", "**/node_modules/**", "vendor/**", "dist/**", "build/**", "out/**",
	"coverage/**", ".next/**", ".cache/**", "target/**", "bin/**", "obj/**", "**/*.min.js", "**/*.map",
	"**/*.lock", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "composer.lock", "go.sum",
}

func set(items ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range items {
		m[s] = true
	}
	return m
}

// Extensions that count as source code (for coverage).
var codeExt = set("ts", "tsx", "js", "jsx", "mjs", "cjs", "vue", "svelte", "php", "go", "py", "rb", "rs", "java", "kt", "kts", "cs", "fs",
	"c", "h", "cc", "cpp", "hpp", "m", "mm", "swift", "dart", "lua", "gd", "gdshader", "shader", "glsl", "hlsl", "wgsl", "sql", "sh", "ps1", "bat",
	"scala", "clj", "ex", "exs", "erl", "hs", "elm", "zig", "nim", "r", "pl", "twig", "blade", "cshtml", "razor")

// Extensions a code ref without a slash may use (`package.json`).
var refExt = func() map[string]bool {
	m := set("json", "yaml", "yml", "toml", "ini", "env", "md", "html", "htm", "css", "scss", "sass", "less", "xml",
		"txt", "tscn", "tres", "godot", "csproj", "sln", "gradle", "properties", "lock", "mod", "sum", "conf", "cfg", "config", "dockerfile", "tf")
	for k := range codeExt {
		m[k] = true
	}
	return m
}()

var binaryExt = set("png", "jpg", "jpeg", "gif", "webp", "ico", "svg", "mp4", "mov", "webm", "mkv", "mp3", "wav", "ogg",
	"ttf", "otf", "woff", "woff2", "eot", "zip", "gz", "7z", "rar", "pdf", "exe", "dll", "so", "dylib", "bin", "dat", "db", "sqlite",
	"psd", "blend", "fbx", "glb", "gltf", "obj", "wasm", "pck", "import")

func extOf(f string) string {
	b := f
	if i := strings.LastIndexByte(b, '/'); i >= 0 {
		b = b[i+1:]
	}
	i := strings.LastIndexByte(b, '.')
	if i > 0 {
		return strings.ToLower(b[i+1:])
	}
	return ""
}

// sourceFiles lists repo text files (git-aware), minus default and extra ignores.
func sourceFiles(extraIgnore []string) []string {
	ignore := append(append([]string{}, defaultIgnore...), extraIgnore...)
	var files []string
	if isGit() {
		out, _ := git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
		for _, f := range strings.Split(out, "\x00") {
			if f != "" {
				files = append(files, f)
			}
		}
	} else {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == root {
				return nil
			}
			r := rel(p)
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "node_modules" || matchAny(r+"/", ignore) || matchAny(r, ignore) {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, r)
			return nil
		})
	}
	out := []string{}
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] || matchAny(f, ignore) || binaryExt[extOf(f)] || !fileExists(filepath.Join(root, f)) {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func lineCount(f string) int {
	b, err := os.ReadFile(filepath.Join(root, f))
	if err != nil || len(b) == 0 {
		return 0
	}
	return strings.Count(string(b), "\n") + 1
}
