---
title: Exports read source from GitHub
summary: A static export doesn't copy the code; the viewer fetches each file from raw.githubusercontent.com at the commit the docs describe.
status: stated
components: [export, viewer]
generated_with: 0.1.0
updated: 2026-10-29
---
## Context

The project needed a live demo on GitHub Pages, and the code panel is the feature most worth
showing. On a static host there's no server to answer the panel's questions.

## Decision

Export precomputes what the server would answer (file list, import links, definitions) into small
JSON files, and the viewer loads source directly from GitHub's raw file host, pinned to a commit
(`tool/export.go#exportSource`). The idea, documenting codedocs itself and loading its files from
GitHub, came from the user.

## Alternatives

- **Copy the source into the export**: works for any host and offline, but duplicates the repo in
  every deploy and serves code the docs don't describe if they drift.
- **Use the GitHub API**: richer, but rate-limited for anonymous visitors.
- **Docs without a code panel**: simplest, but loses the main feature.

## Consequences

- The site is small and the code shown always matches the commit the docs were written against.
- Only works for code hosted publicly on GitHub (or another host given with `--source-url`).
- The viewer carries a JavaScript copy of the glob and definition-ranking rules, kept in step
  with the Go originals by a test that runs both.
