---
title: Glossary
summary: The words this project uses, and what each one means here.
generated_with: 0.1.0
updated: 2026-10-29
---
# Glossary

| Term | Meaning | Where |
|---|---|---|
| Affected | The pages a code change might make wrong, worked out from git and component paths. | `tool/cmds.go#cmdAffected` |
| Code panel | The pane beside a page that shows source files, with import links and go-to-definition. | `tool/assets/index.html#showCode` |
| Code ref | Inline code in a page that names a repo file, optionally with a line, range or symbol. | `tool/pages.go#extractRefs` |
| Component | A major part of the codebase, owning the files its `paths` globs match. | `skills/codedocs/reference/content-model.md` |
| Concern | A pattern that cuts across several components, such as errors or versioning. | `skills/codedocs/reference/content-model.md` |
| Content model | The contract for docs folders: folders, frontmatter, ref and link formats, templates. | `skills/codedocs/reference/content-model.md` |
| Decision | A record of a significant choice, `stated` by the user or `inferred` from the code. | `skills/codedocs/reference/content-model.md` |
| Docs folder | The folder holding `content/`, the config, theme and viewer; usually `docs/`. | `tool/pages.go#findDocsDir` |
| Export | A static copy of the docs whose code panel reads source from GitHub. | `tool/export.go#cmdExport` |
| Flow | A journey through the code, step by step, across components. | `skills/codedocs/reference/content-model.md` |
| Guidance version | The version of the writing guides, stamped on pages as `generated_with`. | `tool/cmds.go#guidanceVersion` |
| Inferred | Worked out from the code rather than told; always marked in the docs. | `skills/codedocs/reference/writing.md` |
| Insight | Something the user told the docs, kept as theirs through every rewrite. | `skills/codedocs/reference/add.md` |
| Language pack | The import patterns and resolution rules for one language. | `tool/links.go#linksFor` |
| Last synced | The commit the docs describe, recorded by `mark-synced`. | `tool/cmds.go#cmdMarkSynced` |
| Launcher | The script the skill runs, which finds or downloads the binary and runs it. | `skills/codedocs/bin/codedocs`, `skills/codedocs/bin/codedocs.ps1` |
| Locked | A page humans own; the agent never rewrites it. | `skills/codedocs/SKILL.md` |
| Manifest | manifest.json: the index of pages, links and resolved symbols the viewer runs on. | `tool/pages.go#buildManifest` |
| Repo context | The file set and per-ecosystem resolution rules imports are resolved against. | `tool/links.go#repoCtx` |
| Skill | The markdown instructions the agent follows for each `/codedocs` command. | `skills/codedocs/SKILL.md` |
| Topic | A page for one subject that spans the structure, written for admins, developers or both. | `skills/codedocs/reference/add.md` |
