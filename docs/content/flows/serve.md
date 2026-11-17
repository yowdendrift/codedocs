---
title: Starting and stopping the viewer
summary: How `codedocs serve` finds the docs, claims a port and serves the viewer, and how `codedocs stop` shuts it down again.
order: 70
components: [cli, server, docs-model]
generated_with: 0.1.0
updated: 2026-11-17
---
# Starting and stopping the viewer

## In short

`codedocs serve` finds the docs folder, checks whether a viewer for that folder is already
running, and if not starts a small web server on `127.0.0.1` and records itself in a state file in
the temp folder. The viewer, its stylesheets and the manifest are served from the binary and
rebuilt per request, so edits to a page show on refresh. `codedocs stop` (also spelled
`shutdown`) reads that state file, sends the server its secret token, and removes the file.

```mermaid
sequenceDiagram
  participant User
  participant CLI as codedocs serve
  participant State as State file in temp
  participant Server as HTTP server
  participant Stop as codedocs stop
  User->>CLI: serve
  CLI->>State: read; is that server answering?
  CLI->>State: write port, token, pid
  CLI->>Server: listen and open the browser
  Server-->>User: viewer, manifest, docs files
  User->>Stop: stop
  Stop->>State: read port and token
  Stop->>Server: POST /api/shutdown with token
  Stop->>State: remove
```

## Step by step

### 1. Find the docs folder and the state file

`tool/serve.go#cmdServe` asks `tool/pages.go#findDocsDir` for the docs folder (`--dir`, else the
first of `docs`, `docs/codebase`, `codebase-docs` that has a docs.config.json). The state file
name comes from `tool/serve.go#stateFile`: a hash of the docs folder path in the temp folder, so
each docs folder has at most one viewer, and `stop` finds it without searching for processes.

### 2. Is one already running?

`tool/serve.go#readState` loads the file, if any. If it exists, `tool/serve.go#serveState.alive`
requests `/api/info` on the recorded port. An answer means a second `serve` prints the running
viewer's address (and opens the browser with `--open`) and returns. No answer means the record is
stale (a crash, a killed process), so the file is deleted and startup continues.

### 3. Write a fresh manifest and pick a port

`tool/pages.go#buildManifest` output is written to manifest.json so a static copy of the folder
stays current. Then `cmdServe` tries to listen on `--port` (default 8780) and, if it is busy, on
the next port, up to ten times. If it moved, it prints that the first port was busy. Only after
listening does it generate a random token and write the state file (pid, port, URL, token, docs
folder, version).

### 4. Open the browser

With `--open`, `tool/serve.go#openBrowser` starts the platform's opener (`rundll32`, `open` or
`xdg-open`) and does not wait for it.

### 5. Serve the viewer

Routes are registered on one mux, behind a guard that only answers requests addressed to
`localhost`, `127.0.0.1` or `[::1]` on the chosen port (so a rebinding attack can't read your
code). The embedded viewer and its assets (/, /viewer.css, /flowview.html,
/flowview.css, /lib/) always come from the running binary, never from the docs folder.
/manifest.json is rebuilt on every request. /flowgraph.json and /flowgraph.js are built
live from the flow pages' flowgraph blocks; with no such blocks they fall back to a
flowgraph.json file in the docs folder. Everything else under `/` goes to
`tool/serve.go#serveDocsFile`, which serves theme.css, `content/*.md` and other files from the
docs folder, refusing paths that escape it. `/api/info` and `/api/files` report the repo's
version, editor and source file list. The code panel's `/api/file` and `/api/definition` are in
[opening code from a page](open-code.md); `/api/callers` serves the flow code view.

### 6. Stop it

`tool/serve.go#cmdStop` finds the state file (every `codedocs-serve-*.json` in the temp folder
with `--all`, otherwise this docs folder's). For each, `tool/serve.go#serveState.shutdown` POSTs
to `/api/shutdown` with the token in `X-Codedocs-Token`. The route answers 403 unless the method
is POST and the token matches; otherwise it replies "stopping" and shuts the server down 100 ms
later. `cmdStop` then waits up to about two seconds for the server to stop answering, so the
binary is unlocked on Windows, and removes the state file. The server also removes its own state
file when it exits, and on Ctrl+C or SIGTERM.

## Flow graph

```flowgraph
`tool/serve.go#cmdServe` | Start the viewer | disk-write
`tool/serve.go#readState` | Look for a running one
`tool/serve.go#serveState.alive` | Is it still answering?
branch | Already running: print its address
`tool/pages.go#buildManifest` | Refresh the manifest
`tool/serve.go#openBrowser` | Open the browser
route ANY / | Serve the viewer
route ANY /manifest.json | Build the manifest per request
`tool/serve.go#serveDocsFile` | Serve docs files
`tool/serve.go#cmdStop` | Stop from another terminal | disk-write
`tool/serve.go#serveState.shutdown` | Ask the server to stop
route ANY /api/shutdown | Check the token
branch | Wrong token or method: refused
```

## When it goes wrong

- "can't listen": ten consecutive ports from `--port` were all taken.
- "The docs viewer for this repo isn't running": no state file, or `stop` was run from a
  different repo (the file is per docs folder). Use `codedocs stop --all` to stop every viewer.
- A stale state file after a crash is harmless: `serve` and `stop` both discard it when the
  recorded port doesn't answer.
- A 403 from the shutdown route means the token didn't match, usually a state file from another
  run.
- A page that's blank on first load with a `file://` URL is not this flow: the viewer needs the
  server's routes.

## Things to know before changing it

- The state file lives in the temp folder, not the repo, so it survives neither a reboot nor a
  temp cleanup; `serve` then simply starts a second server if the first is still running on
  another port (inferred: nothing else tracks the process).
- The server only ever binds `127.0.0.1`. Keep the host guard and the token check together;
  `/api/file` serves repo source, so they are what keeps it private.
- The viewer and flowview.html are embedded (`//go:embed` in `tool/serve.go`), so a new asset
  must be added to that list or it 404s. `/lib/` is cached for a day; flowview.html addresses
  its script by content hash for that reason.
- The server is part of the [server](../components/server.md) component; argument parsing and
  dispatch are in the [CLI](../components/cli.md).
