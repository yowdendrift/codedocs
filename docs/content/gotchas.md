---
title: Gotchas
summary: Traps in this codebase that cost time the first time you hit them.
generated_with: 0.1.0
updated: 2026-10-29
---
# Gotchas

## Editing a repo's index.html changes nothing

`codedocs serve` serves the viewer embedded in the binary, not the index.html in the docs
folder (`tool/serve.go#cmdServe`). That copy exists only for static hosting and is replaced by
`install-viewer`. Change `tool/assets/index.html`, then rebuild.

## A running viewer locks the binary on Windows

`go build` can't replace `codedocs.exe` while a viewer is running. `build.sh` and `build.ps1`
stop viewers first; if you build another way, run `codedocs stop --all` first.

## Git on Windows drops the executable bit

Files committed from Windows are stored as `100644`. For the launcher that breaks plugin installs
on macOS and Linux with "permission denied". Set it with
`git update-index --chmod=+x skills/codedocs/bin/codedocs`; CI fails if it's missing.

## A plain `go build` thinks it's version "dev"

The version comes from `-ldflags` in `build.sh`. A `dev` binary writes `skill_version: dev` into a
docs folder's config and compares as older than everything.

## The viewer has copies of Go logic

Static exports can't call the server, so `tool/assets/index.html#globRe` mirrors
`tool/util.go#globRe`, and the definition ranking in `tool/assets/index.html#Code` mirrors
`tool/defs.go#findDefinition`. Change one, change both.

## `${CLAUDE_SKILL_DIR}` only works in SKILL.md

Claude Code substitutes it in `skills/codedocs/SKILL.md` but not in the guides the agent reads
afterwards. That's why the guides write `<skill>` and SKILL.md defines it.

## Symbol refs resolve to the first good match

`tool/pages.go#resolveSymbol` takes the first strong definition in the file. If a name is
declared twice (an interface and its implementation), use `Type.method` or a line ref to pick
the right one.

## Plugin updates replace the skill folder

Claude Code installs each version into a new cache folder. Anything kept inside the skill folder
is lost on update, including downloaded binaries (re-downloaded) and anything a user saves there.
User settings like a default theme belong in the user's config folder (see
`tool/cmds.go#defaultTheme`).

## An export pins a commit

`codedocs export` points the code panel at one commit. If it isn't pushed, visitors see 404s;
if the docs were written against a different commit, line numbers drift. The command warns about
both.
