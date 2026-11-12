---
title: Code intelligence
summary: Works out what a source file imports, where names are defined and where they end, and roughly where they're called, across several languages, with patterns rather than parsers. Powers clickable imports, go-to-definition, a flow's Code view and find.
order: 40
paths: [tool/links.go, tool/defs.go, tool/find.go, tool/spans.go, tool/callers.go]
generated_with: 0.1.0
updated: 2026-11-12
---
# Code intelligence

## Responsibility

Answers these questions about the repo's source, with no language server or compiler installed:
which files and names a file imports (so imports in the code panel are links), where a name is
defined (Ctrl/Cmd+click in the code panel), where that definition ends and roughly where it's
called from (for the [viewer](viewer.md)'s Code view of a flow), and where a subject lives
(`codedocs find`, used by `/codedocs add`). It doesn't parse code: see
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
set for everything else) and maps each name to where it's declared and where that declaration
ends (`tool/defs.go#Def`), cached for 20 seconds.
`tool/defs.go#findDefinition` ranks candidates: a name imported into the current file wins
(resolved to the exact symbol, or searched within the imported Go package); then a definition
in the same file; then everything else, nearer folders first, capped at 40.

### Symbol extents

`tool/spans.go#symbolSpan` gives the first and last line of a definition, starting from
`tool/pages.go#locateSymbol` and including any doc comment or decorators directly above
(`tool/spans.go#leadStart`). It returns nothing when the symbol was only found as a mention,
because then there's no body to show. Where the body ends is decided by
`tool/spans.go#blockEnd`. For C-like languages, `tool/spans.go#braceEnd` matches brackets while
skipping strings, comments, template literals and regex literals. For Python and GDScript,
`tool/spans.go#indentEnd` follows indentation. A block is capped at 400 lines (`maxSpan`), so
one mismatched brace can't swallow the rest of the file. `tool/spans.go#lineSpan` does the same
for a `file:N` ref: the statement or block that starts on that line.

### Approximate callers

`tool/callers.go#findCallers` lists lines that look like calls to a symbol (`name(`, optionally
with type arguments). It only searches files that can see the symbol: its own file, the files
that import it (`tool/callers.go#importIndex`, built from the same import links and cached for
20 seconds) and, for Go, the rest of its package. It skips the definition itself, comments,
import lines and other declarations of the same name, and stops at 200 results.

### find

`tool/find.go#cmdFind` scores files by how many of the search terms appear in their path and
contents, groups them by folder and lists definitions whose names match. It's the cheap first
step of `/codedocs add`, so the agent reads only the files at the top of the list.

## Interfaces

Used by the [server](server.md) (`/api/file` links, `/api/definition`, `/api/callers`), by the
[docs model](docs-model.md), which stores the extent of every symbol the docs name in the
manifest's `spans`, by [export](export.md), which precomputes the same answers into JSON for
static sites, and by the `find` and `links` commands.

## Watch out for

- Pattern-based resolution is approximate. A name defined in several places returns a list to
  pick from, and unusual import styles (computed paths, re-exports through barrels) may not link.
- Callers are a name match, not a checked reference: two unrelated functions with the same name
  count as each other's calls. That's why the viewer prefixes the count with ≈. Calls through
  an interface, a callback or a re-export are missed.
- Extents come from bracket matching. Code that confuses it (a `/` the heuristic in
  `tool/spans.go#regexStarts` misjudges, or a bracket inside an unusual string syntax) can
  make a body stop early or run on, up to the 400-line cap.
- The static viewer re-implements the ranking in `findDefinition` in JavaScript
  (`tool/assets/index.html#Code`). Changing one without the other makes exported sites behave
  differently from `serve`.
