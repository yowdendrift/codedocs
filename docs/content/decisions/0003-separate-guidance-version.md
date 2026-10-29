---
title: A separate version for page guidance
summary: Pages record the version of the writing guides that produced them, not the tool version, so tool releases don't mark every page outdated.
status: stated
components: [skill, commands]
generated_with: 0.1.0
updated: 2026-10-29
---
## Context

Pages were first stamped with the product version. Every viewer or tool fix then flagged every
page in every repo as outdated, suggesting expensive rewrites when nothing about the pages had
changed.

## Decision

Keep a second version, `skills/codedocs/reference/VERSION`, for how pages are written. Pages store
it in `generated_with` (from `codedocs version --guidance`, see `tool/cmds.go#guidanceVersion`).
Only changes to the templates or writing guide bump it.

## Alternatives

- **Compare major/minor only**: fewer false alarms, but still tied to unrelated changes.
- **No staleness tracking**: simple, but then nobody knows which pages could be improved by a
  better guide.

## Consequences

- "Outdated" now means "a better guide exists for this page", which is worth acting on.
- Anyone changing the guides must remember to bump the guidance version; the changelog's
  **guidance** tag is the reminder. See [versions](../concerns/versioning.md).
