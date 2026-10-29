---
title: Bookkeeping commands
summary: The deterministic commands the skill leans on: inventory, setting up a docs folder, rebuilding the manifest, checking for drift, mapping a change to pages, and tracking the synced commit.
order: 30
paths: [tool/cmds.go]
generated_with: 0.1.0
updated: 2026-10-29
---
# Bookkeeping commands

## Responsibility

The commands that let the agent skip work it would otherwise do by reading files: `stats`,
`install-viewer`, `manifest`, `check`, `affected`, `mark-synced` and `status`. Each one answers a
question cheaply and prints a short, readable answer (most also have `--json`). They never write
documentation prose; they report, and the agent decides. See
[agent and tool](../concerns/agent-and-tool.md) for why the line sits where it does.

## How it works

Every command is a plain function that reads the repo through the [CLI helpers](cli.md) and the
docs through the [docs model](docs-model.md), prints, and exits. Exit codes matter for one of
them: `check` exits 1 when it finds broken refs or links, so CI can run it (this repo's
`.github/workflows/ci.yml` does, on the example docs).

## Parts

### stats

`tool/cmds.go#cmdStats` is the map an agent gets before reading anything: files and lines by
extension and by folder, root files, manifests it recognises (`reManifest`), the largest files
and, in a git repo, the files changed most in six months. It excludes the docs folder and the
config's `ignore` list.

### install-viewer

`tool/cmds.go#cmdInstallViewer` creates or refreshes a docs folder: it writes the built-in
index.html and viewer.css, creates theme.css from `tool/cmds.go#defaultTheme` only if
there isn't one, and creates or updates docs.config.json (recording the tool version as
`skill_version`). With `--offline` it also writes the browser libraries to `lib/`. Without a
folder argument it picks `docs/`, or `docs/codebase/` if `docs/` already holds other files.

### check

`tool/cmds.go#cmdCheck` is the drift detector. For every page it confirms each code ref's file
exists, line numbers are in range and `#Symbol`s resolve; each relative link points at a page;
each `components:` entry names a real component; each component has `paths`. Then it lists code
files no component's `paths` claim, grouped by folder, and pages written with older page
guidance (their `generated_with` is below `tool/cmds.go#guidanceVersion`).

### affected

`tool/cmds.go#cmdAffected` maps a change to pages. It diffs from `last_synced` (or a range the
user gives) to `HEAD`, adds uncommitted and untracked files when no range was given, drops the
docs folder and ignored files, then reports for each changed file the component that owns it,
and for each page why it might need review: it references a changed file, involves a changed
component, or owns changed files. Files nobody owns are listed separately as possible new
components.

### mark-synced and status

`tool/cmds.go#cmdMarkSynced` records `HEAD` as `last_synced` in docs.config.json: the commit
the docs now describe. `tool/cmds.go#cmdStatus` reports the docs folder, the installed tool and
guidance versions against the folder's, page counts, outdated and locked pages, and how many
commits the docs are behind.

### Where the skill folder is

`tool/cmds.go#skillDir` works out the skill folder from the binary's own location (the binary
sits in `<skill>/bin`). `tool/cmds.go#guidanceVersion` reads `reference/VERSION` from there, so
the tool always reports the guidance version of the guides installed beside it.

## Interfaces

Called by the agent through the [skill](skill.md) guides, and by people from a terminal. Outputs
are designed to be read by both: short plain text by default, `--json` for anything that needs
parsing.

## Watch out for

- `check` and `stats` skip the docs folder, but `serve`'s code panel doesn't, so docs files can
  be opened in the panel.
- `affected` needs git and a starting point. With no `last_synced` and no range it stops and
  says so, rather than guessing.
