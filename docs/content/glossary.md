---
title: Glossary
summary: The words this project uses, and what each one means here.
generated_with: 0.1.0
updated: 2026-11-17
---
# Glossary

| Term | Meaning | Where |
|---|---|---|
| Affected | The pages a code change might make wrong, worked out from git and component paths. | `tool/cmds.go#cmdAffected` |
| Callers | The lines that seem to call a symbol: a name match in the files that can see it, so shown as ≈N. | `tool/callers.go#findCallers` |
| Code panel | The pane beside a page that shows source files, with import links and go-to-definition. | `tool/assets/index.html#showCode` |
| Code ref | Inline code in a page that names a repo file, optionally with a line, range or symbol. | `tool/pages.go#extractRefs` |
| Code view | A flow read as code: each step's text followed by the functions it names, with calls that expand in place. | `tool/assets/index.html#showSlice` |
| Component | A major part of the codebase, owning the files its `paths` globs match. | `skills/codedocs/reference/content-model.md` |
| Concern | A pattern that cuts across several components, such as errors or versioning. | `skills/codedocs/reference/content-model.md` |
| Content model | The contract for docs folders: folders, frontmatter, ref and link formats, templates. | `skills/codedocs/reference/content-model.md` |
| Decision | A record of a significant choice, `stated` by the user or `inferred` from the code. | `skills/codedocs/reference/content-model.md` |
| Docs folder | The folder holding `content/`, the config, theme and viewer; usually `docs/`. | `tool/pages.go#findDocsDir` |
| Export | A static copy of the docs whose code panel reads source from GitHub. | `tool/export.go#cmdExport` |
| Flow | A journey through the code, step by step, across components. | `skills/codedocs/reference/content-model.md` |
| Flow graph | flowgraph.json: layers, units and the hops of each flow, built from `flowgraph` blocks and checked against a schema. | `tool/flowgraph.go#validateGraph` |
| Flow view | The 3D picture of the code as glass layers, each flow drawn as a line of light through its units. | `flowview/src/app.ts#start` |
| Flowgraph block | A fenced block on a flow page listing its hops, one hop per line: unit, label and optional effect, separated by pipes. | `tool/flows.go#flowBlocks` |
| Guidance version | The version of the writing guides, stamped on pages as `generated_with`. | `tool/cmds.go#guidanceVersion` |
| Inferred | Worked out from the code rather than told; always marked in the docs. | `skills/codedocs/reference/writing.md` |
| Insight | Something the user told the docs, kept as theirs through every rewrite. | `skills/codedocs/reference/add.md` |
| Language pack | The import patterns and resolution rules for one language. | `tool/links.go#linksFor` |
| Last synced | The commit the docs describe, recorded by `mark-synced`. | `tool/cmds.go#cmdMarkSynced` |
| Launcher | The script the skill runs, which finds or downloads the binary and runs it. | `skills/codedocs/bin/codedocs`, `skills/codedocs/bin/codedocs.ps1` |
| Layer | A tier in the flow view (UI, API, services, data, or the repo's own), from `.codedocs/layers.yml`, a component's `layer`, or file names. | `tool/flows.go#heuristicLayer` |
| Locked | A page humans own; the agent never rewrites it. | `skills/codedocs/SKILL.md` |
| Manifest | manifest.json: the index of pages, links and resolved symbols the viewer runs on. | `tool/pages.go#buildManifest` |
| Reached | A unit no flow step names but a flow calls, found by following calls out of the steps. Shown dimmed in the flow view instead of as a gap. Approximate. | `tool/reach.go#computeReach` |
| Repo context | The file set and per-ecosystem resolution rules imports are resolved against. | `tool/links.go#repoCtx` |
| Skill | The markdown instructions the agent follows for each `/codedocs` command. | `skills/codedocs/SKILL.md` |
| Span | The first and last line of a definition (with its doc comment), stored in the manifest's `spans`. | `tool/spans.go#symbolSpan` |
| Topic | A page for one subject that spans the structure, written for admins, developers or both. | `skills/codedocs/reference/add.md` |
| Unit | A function, method, class or route handler as the flow view draws it; a flow's hops go from unit to unit. | `tool/flows.go#extractUnits` |
