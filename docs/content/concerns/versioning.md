---
title: Versions
summary: Three versions are tracked separately (the product, the page guidance, and what a docs folder was set up with) so each kind of change triggers exactly the work it needs.
order: 20
components: [skill, commands, docs-model, distribution, viewer]
generated_with: 0.1.0
updated: 2026-10-29
---
# Versions

## The pattern

| version | lives in | stamped on | changes when |
|---|---|---|---|
| product | `skills/codedocs/VERSION`, `.claude-plugin/plugin.json`, baked into the binary | releases, docs.config.json as `skill_version` | any release |
| page guidance | `skills/codedocs/reference/VERSION` | each page's `generated_with` | how pages should be written changes |
| docs folder | `skill_version` in docs.config.json | (it is the stamp) | `install-viewer` or `/codedocs upgrade` runs |

`tool/cmds.go#cmdStatus` compares all three: the installed tool against the folder's
`skill_version` ("viewer upgrade available"), and each page's `generated_with` against
`tool/cmds.go#guidanceVersion` (outdated pages). `tool/cmds.go#cmdCheck` lists outdated pages too.

## The rule

- A tool or viewer change bumps only the product version. Existing pages aren't marked outdated,
  because their content didn't get worse.
- A change to templates or the writing guide also bumps the page-guidance version. Pages written
  before it show as outdated, and `/codedocs init` with "Refresh outdated pages" rewrites just
  those.
- Every changelog entry is tagged viewer, content model, guidance or theme, so
  `/codedocs upgrade` knows whether to replace the viewer, migrate pages mechanically, or add theme
  variables.
- The product version must agree everywhere it's written; `scripts/check-versions.sh` enforces it
  in CI and before a release.

## Where it applies

[The skill](../components/skill.md) (guidance), [bookkeeping commands](../components/commands.md)
(status and check), [distribution](../components/distribution.md) (releases and the launcher,
which downloads the binary matching the product version), the [viewer](../components/viewer.md)
(shows the version under the project name).

## Exceptions

A plain `go build` reports `dev`, which compares as older than any real version. Build with
`build.sh` to get the real version.
