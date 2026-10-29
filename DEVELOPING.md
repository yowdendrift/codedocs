# Developing codedocs

## Layout

```
.claude-plugin/
  marketplace.json        makes this repository a Claude Code plugin marketplace
  plugin.json             the plugin (this repository's root is the plugin)
skills/codedocs/          the skill: this folder is what every agent loads
  SKILL.md                entry point and dispatch
  VERSION                 product version; must match plugin.json and the release tag
  REPO                    owner/repo the launchers download release binaries from
  CHANGELOG.md            what changed, tagged viewer / content model / guidance / theme
  reference/              one guide per command, plus content-model.md and writing.md
  reference/VERSION       page-guidance version that pages are stamped with
  agents/openai.yaml      Codex: display name, and no implicit invocation
  bin/codedocs            launcher (sh): runs or downloads the binary for this machine
  bin/codedocs.ps1        the same for PowerShell
  bin/codedocs.exe,       binaries: built by build.sh / build.ps1, or downloaded on first
  bin/codedocs-<os>-<arch>  run; never committed
tool/                     Go source for the binary (standard library only)
  assets/                 the viewer (index.html, viewer.css, default theme.css) and
                          lib/*.js.gz, the browser libraries, embedded into the binary
  vendor.sh               refresh the embedded libraries (needs npm)
docs/                     codedocs' documentation of itself (published as the live demo)
example/                  a tiny fake editor with a full docs set, for trying changes
scripts/check-versions.sh checks VERSION, plugin.json, CHANGELOG and REPO agree
build.sh, build.ps1       build the binaries
install.ps1               link skills/codedocs into ~/.claude/skills for development (Windows)
.github/workflows/        ci.yml (every push), release.yml (tags), pages.yml (the live demo)
```

## Working on it

Install from your clone instead of the plugin, so edits are live. On Windows:

```powershell
.\build.ps1        # skills\codedocs\bin\codedocs.exe (needs Go 1.22+)
.\install.ps1      # junction %USERPROFILE%\.claude\skills\codedocs -> .\skills\codedocs
```

On macOS or Linux: `./build.sh`, then `ln -s "$PWD/skills/codedocs" ~/.claude/skills/codedocs`.

If the plugin is installed as well, `/codedocs` runs your clone and `/codedocs:codedocs` the
plugin. A local build always wins over a download.

The loop:

1. Change something: a guide in `skills/codedocs/reference/`, the viewer in `tool/assets/`, or
   the tool in `tool/*.go`.
2. Rebuild if you touched `tool/` (`.\build.ps1` or `./build.sh`). Both stop running viewers
   first, because Windows locks a running binary.
3. Try it on the example:
   ```sh
   cd example
   ../skills/codedocs/bin/codedocs check
   ../skills/codedocs/bin/codedocs serve --open
   ```
4. Record it in `skills/codedocs/CHANGELOG.md`, tagging each change:
   - **viewer**: `serve` uses it immediately; `/codedocs upgrade` refreshes a repository's static copy.
   - **content model**: `/codedocs upgrade` applies it mechanically.
   - **guidance**: also bump `skills/codedocs/reference/VERSION`. Pages stamped with an older
     guidance version show as outdated; `/codedocs init` with "Refresh outdated pages" rewrites
     only those. Tool and viewer releases leave it alone, so they never mark pages outdated.
   - **theme**: new variables get added to existing themes on upgrade.

## Releasing

Installed copies download the binary for the version in `skills/codedocs/VERSION` from the
GitHub release `v<VERSION>`, so every version users can install needs a release.

1. Set the new version in `skills/codedocs/VERSION` and `.claude-plugin/plugin.json`, and give
   it a `## <version> (<date>)` entry in `CHANGELOG.md`. `scripts/check-versions.sh` checks all
   three agree.
2. Commit and push to `main`.
3. Tag and push the tag:
   ```sh
   git tag v0.5.0
   git push origin v0.5.0
   ```
   `release.yml` checks the versions (and that `skills/codedocs/REPO` names this repository),
   builds all five binaries, and publishes the release with `SHA256SUMS` and the changelog
   entry as notes.

Claude Code only offers users an update when `version` in `plugin.json` changes. Users who
update in the minute or two before the release finishes building get a "try again shortly"
message on first use. To avoid even that, push the tag first and the version bump to `main`
after the release appears; the tag only needs to point at a commit that has the new version.

`./build.sh dist` builds the release assets locally into `dist/`. To test the download path
against them, serve the folder and point the launcher at it:

```sh
mkdir -p /tmp/rel/v0.5.0 && cp dist/* /tmp/rel/v0.5.0/
(cd /tmp/rel && python3 -m http.server 8799) &
CODEDOCS_DOWNLOAD_BASE=http://127.0.0.1:8799 path/to/a/copy/of/skills/codedocs/bin/codedocs version
```

`claude plugin validate .` checks the plugin and marketplace manifests.

## This repo's own docs

`docs/` is codedocs documenting itself, and `pages.yml` publishes it to
https://yowdendrift.github.io/codedocs/ on every push to `main`, with `codedocs export`. After
changing the code, run `/codedocs update` here like in any repo; `check` runs in CI. The demo's
code panel shows the commit in `docs/docs.config.json`'s `last_synced`, so commit the code
first, then run `mark-synced` (which `/codedocs update` does) and commit that.

GitHub Pages must be set to deploy from GitHub Actions once: Settings › Pages › Source: GitHub
Actions.
