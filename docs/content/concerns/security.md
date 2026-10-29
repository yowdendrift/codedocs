---
title: Security
summary: What codedocs exposes and how it's limited: a localhost-only read-only server, a file allowlist with secrets excluded, verified downloads, and exports that publish only what's meant to be public.
order: 30
components: [server, export, distribution]
generated_with: 0.1.0
updated: 2026-10-29
---
# Security

## The pattern

codedocs reads source code and shows it in a browser, and downloads an executable onto users'
machines. Each of those is fenced:

- **The server listens on `127.0.0.1` only** and answers only requests whose Host header is
  `localhost`, `127.0.0.1` or `[::1]` on its port (`tool/serve.go#cmdServe`). That blocks other
  machines and DNS-rebinding pages in the user's own browser.
- **The code panel serves an allowlist, read-only.** Only files in the repo's file set (tracked,
  or untracked and not ignored), not binary, under 2 MB. Paths with `..`, backslashes, colons or a
  leading slash are rejected. `tool/serve.go#secretGlobs` removes `.env*`, keys and certificates
  even if they're tracked.
- **Stopping needs a token.** `/api/shutdown` requires the random token written to the temp-folder
  record, so a web page can't stop the server.
- **Downloads are verified.** The launchers fetch over HTTPS from the project's own release and
  compare against the release's `SHA256SUMS` before keeping the binary
  (`skills/codedocs/bin/codedocs`). A mismatch is refused.
- **Exports leave out machine details.** `tool/export.go#cmdExport` blanks the absolute repo path,
  applies the same secret exclusions, and leaves the source out entirely: visitors fetch it from
  GitHub, which enforces the repository's own visibility.

## The rule

- Anything new that reads files for the browser goes through the same file set and secret
  exclusions.
- Nothing listens on a public interface.
- Nothing downloaded runs without a checksum check.

## Where it applies

[Viewer server](../components/server.md), [static export](../components/export.md),
[distribution](../components/distribution.md).

## Exceptions

The checksum file comes from the same release as the binary, so it protects against corrupted or
swapped downloads, not against a compromised GitHub account. The binaries aren't code-signed.
