---
title: Patterns, not parsers, for code intelligence
summary: Import links, symbol lines and go-to-definition use regular expressions per language instead of language servers or parsers.
status: stated
components: [code-intelligence, docs-model]
generated_with: 0.1.0
updated: 2026-10-29
---
## Context

The code panel needed imports to be clickable and names to jump to their definitions, across the
languages people actually use (JavaScript and TypeScript, PHP, Go, Python, GDScript…), on any
machine, without setup. The ask was for "the simple version" first.

## Decision

Each language gets a small pack of regular expressions for its import statements and declarations,
plus resolution rules read from the repo's own config (tsconfig paths, composer PSR-4, go.mod,
project.godot). See `tool/links.go#linksFor`, `tool/defs.go#defPatterns` and
`tool/pages.go#resolveSymbol`.

## Alternatives

- **Language servers** (via LSP): precise, but one per language, each needing installation and a
  warm-up, and heavy for a docs viewer.
- **Parsers such as tree-sitter**: accurate syntax, but grammars per language and cgo or large
  binaries, against [0001](0001-go-single-binary.md).
- **Per-language subskills that the agent runs**: discussed, but it spends tokens on something a
  program can do.

## Consequences

- Works for any repo immediately, and new languages are a few patterns away.
- Results are approximate: a name defined in several places returns a list, and computed imports
  or re-export barrels may not resolve. `/codedocs links` reports resolution rates per language and
  lets the agent add alias rules.
- The same logic is mirrored in the viewer for static exports, so the two must change together.
