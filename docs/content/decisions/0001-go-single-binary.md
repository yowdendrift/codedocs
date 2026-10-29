---
title: One Go binary, standard library only
summary: The tool is a single executable per platform with the viewer embedded, so using codedocs needs no Node, Python or package install.
status: stated
components: [cli, server, viewer, distribution]
generated_with: 0.1.0
updated: 2026-10-29
---
## Context

The first versions of the tool were Node scripts. That made every user install Node, and made
"run the viewer" depend on a working Node setup on each machine, including Windows.

## Decision

Rewrite the tool in Go as one statically linked executable per platform, using only the standard
library. The viewer, its stylesheet, the default theme and the browser libraries are embedded with
`embed` (`tool/serve.go#assets`), so the binary is the whole product apart from the skill's
markdown.

## Alternatives

- **Keep Node**: familiar for a JavaScript developer, but a runtime prerequisite for every user.
- **Go with third-party modules** (a tree-sitter binding, a markdown parser): richer analysis,
  but cgo or larger binaries, and a dependency chain to keep current. Module downloads were also
  unavailable in the environment the tool was first built in.

## Consequences

- Install is one file (downloaded automatically by the launcher), on Windows, macOS and Linux.
- Builds are fast and reproducible: `CGO_ENABLED=0`, nothing to fetch.
- Code analysis is pattern-based rather than parsed; see
  [0002](0002-regex-code-intelligence.md).
- The frontmatter parser is hand-written and deliberately limited
  (`tool/pages.go#parseFrontmatter`), which is why the content model allows no nested YAML.
