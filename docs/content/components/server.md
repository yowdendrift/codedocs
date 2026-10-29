---
title: Viewer server
summary: codedocs serve: a localhost web server for the viewer, the docs and a read-only code API, plus the bookkeeping that lets stop find a running server from any session.
order: 50
paths: [tool/serve.go]
generated_with: 0.1.0
updated: 2026-10-29
---
# Viewer server

## Responsibility

Serves the [viewer](viewer.md) and the docs folder on `localhost`, and answers the code panel's
questions: list files, return a file with its import links and owning components, find a
definition. It's the only part of codedocs that opens a network port, so it's also where most of
the [security rules](../concerns/security.md) live. It doesn't render anything; the browser
does.

## How it works

`tool/serve.go#cmdServe` rebuilds manifest.json on disk, picks a port (8780, or the next free
one of ten), and starts a standard-library HTTP server bound to `127.0.0.1`. The viewer files
and libraries come from the binary itself (`tool/serve.go#assets`, embedded at build time), so
the copy of index.html in the docs folder is ignored while serving and a viewer fix reaches
every repo the moment the binary is updated. The manifest is rebuilt on every request, so edits
to the markdown show on refresh.

Each server records itself in a small JSON file in the temp folder, keyed by a hash of the docs
folder. That's how a second `serve` knows one is already running and just prints its address,
and how `stop` finds it without killing processes.

## Parts

### Routes

- `/`, /viewer.css, `/lib/*`: the embedded viewer and libraries (served gzipped when the
  browser accepts it).
- /manifest.json: built fresh per request.
- Anything else: a file from the docs folder via `tool/serve.go#serveDocsFile`, confined to it.
- `/api/info`, `/api/files`, `/api/file`, `/api/definition`: the code panel API.
- `/api/shutdown`: stop, if the request carries the token from the state file.

### The code API

`/api/file` checks the path is clean and in the repo's file set (`tool/serve.go#fileSet`,
refreshed every few seconds), is under 2 MB and isn't binary, then returns the text with CRLF
normalised, the line a requested symbol is on (via `tool/pages.go#resolveSymbol`), the
components whose `paths` include it, and its import links from
[code intelligence](code-intelligence.md). `/api/definition` delegates to
`tool/defs.go#findDefinition`.

### Running-server records and stop

`tool/serve.go#serveState` is what's written to the temp file: PID, port, URL, a random token,
the docs folder and project. `tool/serve.go#serveState.alive` and
`tool/serve.go#serveState.shutdown` talk to a recorded server over HTTP;
`tool/serve.go#cmdStop` asks this repo's server (or, with `--all`, every recorded one) to shut
down, then waits briefly for it to exit so its binary is unlocked on Windows.

## Interfaces

Talks to the browser only. The JSON shapes of `/api/*` responses are the contract with the
viewer's `tool/assets/index.html#Code` object; [export](export.md) writes static files that the
same object reads instead.

## State and data

Nothing persistent beyond the temp-folder record, which is removed on clean shutdown. A crashed
server leaves a stale record; the next `serve` or `stop` notices it doesn't answer and deletes it.

## Watch out for

- The Host-header check rejects anything not addressed to `localhost`, `127.0.0.1` or `[::1]` on
  the right port. Opening the viewer through a LAN IP or a tunnel fails by design.
- `serve` runs until stopped. Agents must start it in the background; see the `serve` section of
  `skills/codedocs/SKILL.md`.
