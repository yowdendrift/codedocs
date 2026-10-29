---
title: Code intelligence
summary: Works out what a source file imports and where names are defined, across several languages, with patterns rather than parsers. Powers clickable imports, go-to-definition and find.
order: 40
paths: [tool/links.go, tool/defs.go, tool/find.go, tool/parity_test.go]
generated_with: 0.1.0
updated: 2026-10-29
---
# Code intelligence

## Responsibility

Answers three questions about the repo's source, with no language server or compiler installed:
which files and names a file imports (so imports in the code panel are links), where a name is
defined (Ctrl/Cmd+click in the code panel), and where a subject lives (`codedocs find`, used by
`/codedocs add`). It doesn't parse code: see
[the decision](../decisions/0002-regex-code-intelligence.md).

## How it works

Everything starts from a repo context, a snapshot of the file list plus each ecosystem's own
resolution rules read from the repo (tsconfig path aliases, composer PSR-4 namespaces, the Go
module name, the Godot project root, package.json dependencies). Per-language "packs" then scan
a file's text for import statements with regular expressions and resolve each one against that
context. Definitions come from a separate name index built with per-language declaration
patterns.

## Parts

### Repo context

`tool/links.go#buildRepoCtx` builds `tool/links.go#repoCtx`: the file set, a lower-case lookup
(for case-insensitive file systems), files by base name, every folder, and the alias and root
rules above, plus anything in the docs config's `links` section. `tool/links.go#getRepoCtx`
caches it for a few seconds so the [server](server.md) doesn't rebuild it per request.

### Language packs

`tool/links.go#linksFor` picks a pack by extension (`langByExt`) and returns
`tool/links.go#fileLinks`: the clickable spans (`tool/links.go#Link`: line, text, which
occurrence, target path, optional symbol), the names each import brings in, and imports that
look local but didn't resolve. Packs exist for JavaScript/TypeScript (`tool/links.go#jsLinks`,
including Vue and Svelte), PHP (`tool/links.go#phpLinks`), Go (`tool/links.go#goLinks`, where an
import is a folder), Python (`tool/links.go#pyLinks`), Godot (`tool/links.go#godotLinks`) and
CSS (`tool/links.go#cssLinks`). `tool/links.go#cmdLinks` reports how well each language
resolves in a repo, which `/codedocs links` uses to decide whether to add alias rules.

### Definition index

`tool/defs.go#defIndex` scans every source file once with patterns from
`tool/defs.go#defPatterns` (specific sets for JS, PHP, Go, Python and GDScript, and a generic
set for everything else) and maps each name to where it's declared, cached for 20 seconds.
`tool/defs.go#findDefinition` gathers the candidates and the current file's imports, resolved by
`tool/defs.go#resolveImports` to a file and line (or a Go package folder), and hands them to
`tool/defs.go#rankDefinitions`: a name imported into the current file wins (searched within the
package for a Go import); then a definition in the same file; then everything else, nearer
folders first, capped at 40. The ranking is a pure function of its inputs so it can be tested
against the viewer's copy.

### find

`tool/find.go#cmdFind` scores files by how many of the search terms appear in their path and
contents, groups them by folder and lists definitions whose names match. It's the cheap first
step of `/codedocs add`, so the agent reads only the files at the top of the list.

## Interfaces

Used by the [server](server.md) (`/api/file` links, `/api/definition`), by
[export](export.md), which precomputes the same answers into JSON for static sites, and by the
`find` and `links` commands.

## Watch out for

- Pattern-based resolution is approximate. A name defined in several places returns a list to
  pick from, and unusual import styles (computed paths, re-exports through barrels) may not link.
- The static viewer has JavaScript copies of `rankDefinitions` and the glob rules, so exported
  sites behave like `serve`. `tool/parity_test.go#TestViewerParity` runs both versions on the same
  cases and fails on any difference, so change them together and run `go test ./...` (it needs
  Node; CI always runs it).
