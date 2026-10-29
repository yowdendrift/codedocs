---
title: Distribution
summary: How codedocs gets onto a machine and stays current: the plugin and marketplace manifests, the launchers that download the right binary on first run, the build scripts, and the CI, release and Pages workflows.
order: 90
paths: [skills/codedocs/bin/**, build.sh, build.ps1, install.ps1, scripts/**, .github/**, .claude-plugin/**]
generated_with: 0.1.0
updated: 2026-10-29
---
# Distribution

## Responsibility

Packaging and delivery: making the repository installable as a Claude Code plugin, building
binaries for five platforms, publishing them as GitHub releases, and getting the right one onto
a user's machine without them installing anything. Also the Pages workflow that publishes this
repo's own docs as a live demo.

## How it works

The repository is both a plugin and a one-plugin marketplace: `.claude-plugin/marketplace.json`
lists the plugin with `"source": "./"`, and `.claude-plugin/plugin.json` describes it. Claude Code
clones the repo, copies it into its plugin cache, and loads `skills/codedocs/`.

Binaries aren't in git ([why](../decisions/0004-binaries-from-releases.md)). The skill always
runs `skills/codedocs/bin/codedocs`, a small shell launcher. If the binary for this machine is
already next to it (a local build, or an earlier download) it runs it; otherwise it downloads the
one for the skill's version from the GitHub release, checks its SHA-256, keeps it, and runs it.

## Parts

### Launchers

`skills/codedocs/bin/codedocs` (POSIX sh, also used by Git Bash on Windows) maps `uname` to a
release asset name, downloads with curl or wget from
`https://github.com/<REPO>/releases/download/v<VERSION>/`, verifies against the release's
`SHA256SUMS`, and `exec`s the binary. `skills/codedocs/REPO` names the repository and
`skills/codedocs/VERSION` the version. `CODEDOCS_DOWNLOAD_BASE` points it at a mirror (or a
local test server); `CODEDOCS_NO_DOWNLOAD=1` makes it run a local binary or nothing.
`skills/codedocs/bin/codedocs.ps1` does the same in PowerShell for Windows without bash.

### Build scripts

`build.sh` builds for this machine, `all` five targets into `skills/codedocs/bin`, or `dist` to
produce release assets and `SHA256SUMS` in `dist/`. It stops any running viewer first (a running
`codedocs.exe` is locked on Windows). `build.ps1` is the Windows equivalent. `install.ps1` links
`skills/codedocs` into `~/.claude/skills` for working on codedocs itself. All builds are
`CGO_ENABLED=0` with the version baked in through `-ldflags`.

### Workflows

- `.github/workflows/ci.yml`, on every push and pull request: version consistency, the
  launcher's executable bit, `go vet`, `go test` (including the viewer parity test), all five
  builds, and `check` on the example docs and this repo's docs.
- `.github/workflows/release.yml`, on a `v*` tag: the same checks plus tag-matches-version and
  `REPO`-names-this-repository, then `build.sh dist` and a GitHub release with the changelog
  entry as notes.
- `.github/workflows/pages.yml`, on pushes to `main`: builds the tool, exports `docs/` with
  `codedocs export` and deploys it to GitHub Pages.

`scripts/check-versions.sh` is the shared guard: `skills/codedocs/VERSION`, `.claude-plugin/plugin.json`, the
changelog and (in CI) the tag and `REPO` must agree.

## Interfaces

Wraps the [CLI](cli.md) binary for the [skill](skill.md). Depends on GitHub Releases being
reachable from the user's machine on first run.

## Watch out for

- Every version users can install needs a release, because the launcher downloads the binary
  matching `VERSION`. Push the tag; the release workflow does the rest.
- Git on Windows records files as not executable. The launcher must be `100755` in git or
  plugin installs fail on macOS and Linux; CI checks this. Fix with
  `git update-index --chmod=+x skills/codedocs/bin/codedocs`.
- Claude Code only offers an update when `version` in `.claude-plugin/plugin.json` changes.
