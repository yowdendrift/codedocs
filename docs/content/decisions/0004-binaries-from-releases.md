---
title: Binaries come from releases, not git
summary: The five platform binaries are built by CI and attached to GitHub releases; the launcher downloads the right one on first run.
status: stated
components: [distribution]
generated_with: 0.1.0
updated: 2026-10-29
---
## Context

Installing as a Claude Code plugin means Claude Code clones the repository. The binaries are about
8 MB each, five platforms per version. Committing them would add about 40 MB to the repository's
history on every release, and every user would download all five.

## Decision

Keep binaries out of git. Tagged releases build them (`.github/workflows/release.yml`) and publish
them with a `SHA256SUMS` file. The launcher (`skills/codedocs/bin/codedocs`) downloads the one for
the machine and the skill's version on first use, verifies it, and keeps it.

## Alternatives

- **Commit the binaries**: works offline from the first command, but bloats every clone and needs
  care with line endings and executable bits in git.
- **Git LFS**: Claude Code's plugin clone doesn't fetch LFS content.
- **Build on install**: needs Go on every machine.

## Consequences

- The repository stays small and installs are fast.
- First use needs network access to GitHub; sandboxed agents may need to approve it.
- Every installable version needs a release, and a release must finish before users run that
  version. See [releasing](../flows/release.md).
