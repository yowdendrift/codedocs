---
title: Static export
summary: codedocs export: writes the docs as a static site any web host can serve, with a working code panel that reads source from GitHub at the commit the docs describe.
order: 60
paths: [tool/export.go]
generated_with: 0.1.0
updated: 2026-10-29
---
# Static export

## Responsibility

Produces a folder you can put on GitHub Pages (or any static host) that behaves like
`codedocs serve`, code panel included, with no server. It doesn't copy the source code: it
records where to fetch it and precomputes the answers the [server](server.md) would otherwise
work out per request. This repo's own live demo is built with it; see
[publishing a static site](../flows/publishing.md).

## How it works

The export is the viewer, the docs, and a `code/` folder:

```
index.html, viewer.css, theme.css, lib/   the viewer and its libraries
manifest.json, content/                   the docs ("static": true, no machine paths)
code/index.json                           source URL, web URL, commit, file list
code/defs.json                            name -> definitions, loaded on first Ctrl+click
code/meta/<path>.json                     per file: import links, where imported names live
```

The source URL is `raw.githubusercontent.com/<owner>/<repo>/<commit>/<subfolder>/`. Pinning a
commit rather than a branch means line numbers in the docs always match the code shown, and
GitHub serves raw files to any page (it allows cross-origin requests), so the browser can fetch
them directly.

## Parts

### Choosing the source

`tool/export.go#exportSource` takes `--source-url`/`--web-url` as given; otherwise it uses
`--github OWNER/REPO` or parses the `origin` remote with `reGitHubRemote`. The commit is
`--ref`, else the docs' `last_synced` (the commit they describe), else `HEAD`. If the repo root
is a subfolder of the git repository, that prefix is added to the URLs. Without a GitHub source
the export still works, just without a code panel.

### Writing the site

`tool/export.go#cmdExport` refuses a non-empty folder that doesn't look like an earlier export,
then writes the embedded viewer and libraries, the theme, `content/` and a manifest with
`repo_root_abs` blanked, `editor` set to `none` and `static` set. For the code panel it takes
the same file set as the server, minus the docs folder, files over 2 MB and binary files, and
writes `tool/export.go#codeIndex`. Definitions are stored compactly as
`name -> [[file index, line], …]`. Per-file `tool/export.go#codeMeta` holds the links from
`tool/links.go#linksFor`, imports resolved to a line (`tool/export.go#exportImport`), and the
lines of symbols those links point at, so following an import lands on the right line without a
server. Files with nothing to say get no meta file; code/index.json lists the ones that do, so
the viewer never requests a missing file.

### Warnings

It warns when the pinned commit isn't on any remote branch (the code panel would show "not
found" until it's pushed), and when the docs' `last_synced` differs from the commit being shown.

## Interfaces

Reads through the [docs model](docs-model.md) and [code intelligence](code-intelligence.md).
Its output is read by the [viewer](viewer.md)'s static mode. The Pages workflow
(`.github/workflows/pages.yml`) runs it on every push to `main`.

## Watch out for

- An export publishes the repo's file list and its source URLs. For a private repository the
  source URLs won't load for visitors, and the file list still reveals names: only export
  what's meant to be public.
- The export is a snapshot. After `/codedocs update`, export again.
