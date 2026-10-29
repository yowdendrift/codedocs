package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed assets/index.html assets/viewer.css assets/theme.css assets/lib/*.gz
var assets embed.FS

const maxCodeFile = 2 << 20 // 2 MB

// Files the code panel never serves, even if tracked.
var secretGlobs = []string{"**/.env", "**/.env.*", ".env", ".env.*", "**/*.pem", "**/*.key", "**/*.p12", "**/*.pfx", "**/id_rsa*", "**/id_ed25519*"}

type fileSet struct {
	mu    sync.Mutex
	at    time.Time
	files []string
	set   map[string]bool
}

func (fs *fileSet) get() ([]string, map[string]bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if time.Since(fs.at) > 3*time.Second {
		fs.files = sourceFiles(secretGlobs)
		fs.set = map[string]bool{}
		for _, f := range fs.files {
			fs.set[f] = true
		}
		fs.at = time.Now()
	}
	return fs.files, fs.set
}

// ---------- running-server state ----------
// Each running server records itself in the temp folder, so `stop` (and a second
// `serve`) can find it on any OS without killing processes.

type serveState struct {
	PID     int    `json:"pid"`
	Port    string `json:"port"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	DocsDir string `json:"docs_dir"`
	Project string `json:"project"`
	Version string `json:"version"`
	Started string `json:"started"`
}

func stateFile(docsDir string) string {
	h := sha1.Sum([]byte(strings.ToLower(posix(docsDir))))
	return filepath.Join(os.TempDir(), "codedocs-serve-"+hex.EncodeToString(h[:6])+".json")
}

func readState(f string) *serveState {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil
	}
	st := &serveState{}
	if jsonUnmarshal(b, st) != nil || st.Port == "" {
		return nil
	}
	return st
}

var localClient = &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}

func (st *serveState) alive() bool {
	r, err := localClient.Get("http://127.0.0.1:" + st.Port + "/api/info")
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}

// shutdown asks a server to stop. false = it wasn't reachable (stale record).
func (st *serveState) shutdown() bool {
	req, _ := http.NewRequest("POST", "http://127.0.0.1:"+st.Port+"/api/shutdown", nil)
	req.Header.Set("X-Codedocs-Token", st.Token)
	r, err := localClient.Do(req)
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}

func cmdStop(o options) {
	var files []string
	if o.all {
		files, _ = filepath.Glob(filepath.Join(os.TempDir(), "codedocs-serve-*.json"))
	} else {
		files = []string{stateFile(findDocsDir(o, true))}
	}
	stopped := 0
	for _, f := range files {
		st := readState(f)
		if st == nil {
			continue
		}
		if st.shutdown() {
			stopped++
			fmt.Printf("Stopped the docs viewer for %s (%s)\n", st.Project, st.URL)
			// Wait briefly for the process to exit, so its binary is unlocked (Windows).
			for i := 0; i < 20 && st.alive(); i++ {
				time.Sleep(100 * time.Millisecond)
			}
		}
		os.Remove(f)
	}
	if stopped == 0 {
		if o.all {
			fmt.Println("No docs viewers were running.")
		} else {
			fmt.Println("The docs viewer for this repo isn't running.")
		}
	}
}

func cmdServe(o options) {
	docsDir := findDocsDir(o, true)
	cfg := loadConfig(docsDir)
	sf := stateFile(docsDir)
	if st := readState(sf); st != nil {
		if st.alive() {
			fmt.Printf("Docs viewer for %s is already running: %s  (stop it with: codedocs stop)\n", st.Project, st.URL)
			if o.open {
				openBrowser(st.URL)
			}
			return
		}
		os.Remove(sf)
	}
	// Fresh manifest on disk too, so a static copy of the folder stays current.
	writeJSON(filepath.Join(docsDir, "manifest.json"), buildManifest(docsDir, cfg))

	var ln net.Listener
	var err error
	port := o.port
	for i := 0; i < 10; i++ {
		if ln, err = net.Listen("tcp", "127.0.0.1:"+port); err == nil {
			break
		}
		var p int
		fmt.Sscanf(port, "%d", &p)
		port = fmt.Sprint(p + 1)
	}
	if err != nil {
		die("can't listen: " + err.Error())
	}
	files := &fileSet{}
	tok := make([]byte, 16)
	rand.Read(tok)
	token := hex.EncodeToString(tok)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	allowedHosts := set("localhost:"+port, "127.0.0.1:"+port, "[::1]:"+port)

	mux := http.NewServeMux()
	send := func(w http.ResponseWriter, ctype string, b []byte) {
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(b)
	}
	jsonResp := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		w.Write(marshal(v))
	}

	// Viewer: always the version built into this binary.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			serveDocsFile(w, r, docsDir)
			return
		}
		send(w, "text/html; charset=utf-8", readAsset("index.html"))
	})
	mux.HandleFunc("/viewer.css", func(w http.ResponseWriter, r *http.Request) {
		send(w, "text/css; charset=utf-8", readAsset("viewer.css"))
	})
	mux.HandleFunc("/lib/", func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.URL.Path)
		b, err := assets.ReadFile("assets/lib/" + name + ".gz")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=86400")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(b)
			return
		}
		w.Write(gunzip(b))
	})
	// Live manifest: rebuilt on every load, so edits show on refresh.
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		send(w, "application/json; charset=utf-8", marshal(buildManifest(docsDir, loadConfig(docsDir))))
	})

	// Stop on request from `codedocs stop` (needs the token from the state file).
	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Codedocs-Token") != token {
			jsonResp(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		jsonResp(w, 200, map[string]string{"status": "stopping"})
		go func() { time.Sleep(100 * time.Millisecond); srv.Shutdown(context.Background()) }()
	})

	// ---- code panel API ----
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		fl, _ := files.get()
		jsonResp(w, 200, map[string]any{"version": version, "root": posix(root), "editor": cfg.Editor, "files": len(fl), "codePanel": true})
	})
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		fl, _ := files.get()
		globs := r.URL.Query()["glob"]
		out := []string{}
		for _, f := range fl {
			if len(globs) == 0 || matchAny(f, globs) {
				out = append(out, f)
			}
		}
		sort.Strings(out)
		jsonResp(w, 200, map[string]any{"files": out})
	})
	// Go to definition: the file's own imports first, then the name index.
	mux.HandleFunc("/api/definition", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		name := q.Get("name")
		if name == "" || len(name) > 200 {
			jsonResp(w, 400, map[string]string{"error": "bad name"})
			return
		}
		jsonResp(w, 200, map[string]any{"name": name, "matches": findDefinition(getRepoCtx(docsDir), q.Get("from"), name)})
	})
	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		p := q.Get("path")
		clean := path.Clean(strings.TrimPrefix(p, "./"))
		if p == "" || strings.Contains(p, "\\") || strings.Contains(p, ":") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") {
			jsonResp(w, 400, map[string]string{"error": "bad path"})
			return
		}
		_, inSet := files.get()
		if !inSet[clean] {
			jsonResp(w, 404, map[string]string{"error": "not a source file in this repo (missing, ignored or binary): " + clean})
			return
		}
		full := filepath.Join(root, filepath.FromSlash(clean))
		st, err := os.Stat(full)
		if err != nil {
			jsonResp(w, 404, map[string]string{"error": "not found: " + clean})
			return
		}
		if st.Size() > maxCodeFile {
			jsonResp(w, 413, map[string]string{"error": fmt.Sprintf("file too large to show (%d KB)", st.Size()/1024)})
			return
		}
		b, err := os.ReadFile(full)
		if err != nil {
			jsonResp(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
			jsonResp(w, 415, map[string]string{"error": "binary file"})
			return
		}
		text := strings.ReplaceAll(string(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))), "\r\n", "\n")
		resp := map[string]any{"path": clean, "text": text, "lines": strings.Count(text, "\n") + 1}
		if sym := q.Get("symbol"); sym != "" {
			fileCache.Delete(clean)
			resp["symbolLine"] = resolveSymbol(clean, sym)
		}
		var owners []map[string]string
		for _, pg := range loadPages(docsDir) {
			if pg.Kind == "component" && matchAny(clean, pg.list("paths")) {
				owners = append(owners, map[string]string{"id": pg.ID, "title": pg.Title})
			}
		}
		resp["components"] = owners
		if fl := linksFor(getRepoCtx(docsDir), clean, text); len(fl.Links) > 0 {
			resp["links"] = fl.Links
		}
		jsonResp(w, 200, resp)
	})

	// Only answer requests addressed to localhost (blocks DNS-rebinding tricks).
	guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		mux.ServeHTTP(w, r)
	})

	url := "http://localhost:" + port + "/"
	writeJSON(sf, serveState{PID: os.Getpid(), Port: port, URL: url, Token: token, DocsDir: posix(docsDir),
		Project: cfg.Project, Version: version, Started: time.Now().Format(time.RFC3339)})
	defer os.Remove(sf)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; srv.Shutdown(context.Background()) }()

	fmt.Printf("Docs viewer for %s: %s  (stop with Ctrl+C or: codedocs stop)\n", cfg.Project, url)
	if port != o.port {
		fmt.Printf("(port %s was busy)\n", o.port)
	}
	if o.open {
		openBrowser(url)
	}
	srv.Handler = guard
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Remove(sf)
		die(err.Error())
	}
	fmt.Println("Docs viewer stopped.")
}

// serveDocsFile serves theme.css, content/*.md and other files from the docs folder.
func serveDocsFile(w http.ResponseWriter, r *http.Request, docsDir string) {
	clean := path.Clean("/" + r.URL.Path)
	full := filepath.Join(docsDir, filepath.FromSlash(clean))
	if !strings.HasPrefix(full, docsDir) || !fileExists(full) || strings.HasSuffix(clean, ".json.tmp") {
		http.NotFound(w, r)
		return
	}
	ctype := mime.TypeByExtension(path.Ext(clean))
	switch path.Ext(clean) {
	case ".md":
		ctype = "text/markdown; charset=utf-8"
	case ".css":
		ctype = "text/css; charset=utf-8"
	case ".json":
		ctype = "application/json; charset=utf-8"
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	b, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
