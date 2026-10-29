---
title: Docs model
summary: Reads the docs folder (config, pages, frontmatter, code refs, links) and builds manifest.json, the index the viewer runs on.
order: 20
paths: [tool/pages.go]
generated_with: 0.1.0
updated: 2026-10-29
---
# Docs model

## Responsibility

Everything the tool knows about a docs folder comes through here: where it is, its config, its
pages and what they contain. It turns that into manifest.json. It doesn't decide what's wrong
with the docs (that's `check` in [commands](commands.md)) and doesn't write pages (the agent does,
following the [skill](skill.md)).

## How it works

A docs folder is plain files: docs.config.json plus markdown under `content/`. The folder a
page sits in gives its kind (`components/` → component), and a few lines of simple frontmatter
give its title, summary and links to other pages. Code refs are inline code that looks like a
repo path, optionally followed by `:line` or `#Symbol`.

The model reads all of that on every call; nothing is cached between runs. Pages are small and
few, so a full re-read is cheap and the manifest can never be stale relative to the markdown.

## Parts

### Finding the folder and its config

`tool/pages.go#findDocsDir` uses `--dir` if given, otherwise the first of `docs/`,
`docs/codebase/` and `codebase-docs/` that holds a docs.config.json.
`tool/pages.go#loadConfig` reads `tool/pages.go#Config`: project name, editor, `ignore` globs,
the `last_synced` commit, the version the folder was set up with, and optional
`tool/pages.go#LinkConfig` rules for the code panel.

### Pages and frontmatter

`tool/pages.go#loadPages` walks `content/` and builds a `tool/pages.go#Page` per markdown file.
`tool/pages.go#parseFrontmatter` is deliberately not YAML: one `key: value` per line, lists as
`[a, b]`, numbers and booleans recognised, nothing nested. That's all the content model allows,
and it keeps the binary dependency-free.

### Refs, links and headings

`tool/pages.go#extractRefs` finds code refs with `reRef` after `tool/pages.go#stripCode` has
blanked fenced code blocks (examples inside ``` aren't refs) while keeping line numbers, so
`check` can report the right line. `tool/pages.go#extractLinks` finds relative `.md` links, and
`tool/pages.go#headings` the `##`/`###` headings the viewer uses for its outline and the map.

### Symbol resolution

`tool/pages.go#resolveSymbol` turns `file#Symbol` (or `file#Class.method`) into a line number
without a parser. For each dotted part it tries, from the previous part's line onward: a
"strong" definition (`class X`, `func X`, `def X`…), then a looser definition pattern
(`X = function`, `X(…) {`), then the first plain mention. Lines like `import X` never count as
definitions. It's language-agnostic and usually right; see
[the decision](../decisions/0002-regex-code-intelligence.md) for the trade-off.

### The manifest

`tool/pages.go#buildManifest` produces `tool/pages.go#Manifest`: one `tool/pages.go#Entry` per
page, sorted by kind, `order` and title, with the refs it mentions and its headings. It also
computes the reverse links (which flows, concerns, decisions and topics involve each component),
so pages only ever declare links in one direction. Every `file#Symbol` ref is resolved to a line
here, so the viewer can link straight to the right line without asking the server.

## Interfaces

Used by every command that touches the docs, by the [server](server.md) (the live manifest) and
by [export](export.md). The manifest's JSON shape is the contract with the
[viewer](viewer.md): renaming a field means changing `tool/assets/index.html` too.

## State and data

None of its own. It reads docs.config.json and `content/`; only `install-viewer`,
`mark-synced`, `manifest`, `serve` and `export` write anything, and they write through
`tool/util.go#writeJSON`.

## Watch out for

- manifest.json includes `repo_root_abs`, an absolute path on the machine that built it, for
  editor links. [Export](export.md) blanks it; a docs folder committed to git carries it.
- A page's kind comes from its folder, so a page moved between folders changes kind.
