---
title: CLI and shared helpers
summary: The codedocs executable's entry point, its option parsing, and the helpers every command uses to find files, run git and write JSON.
order: 10
paths: [tool/main.go, tool/util.go, tool/files.go, tool/go.mod]
generated_with: 0.1.0
updated: 2026-10-29
---
# CLI and shared helpers

## Responsibility

Turns `codedocs <command> [options]` into a call to one command function, and provides the small
helpers the commands share: listing the repo's files, running git, matching globs, comparing
versions and writing JSON safely. It holds no documentation logic of its own; that lives in the
[docs model](docs-model.md) and the [commands](commands.md).

## How it works

`main` reads the working directory into the package-level `root` (every path in the tool is
relative to it), parses the options once, and switches on the command name. There's no
framework: one `options` struct carries every flag any command accepts, and each command reads
the fields it cares about. The tool is meant to be run from the repo root, by the skill or by
hand.

## Parts

### Entry point and options

`tool/main.go#main` dispatches; `tool/main.go#parseArgs` fills `tool/main.go#options` from flags
(`--dir`, `--port`, `--json`, `--github`, …) and collects everything else as positional
arguments. The usage text in `tool/main.go#usage` is the only command reference the binary
prints. The version is a string set at build time with `-ldflags "-X main.version=…"`
(see `build.sh`); a plain `go build` reports `dev`.

### Repo files

`tool/files.go#sourceFiles` is the one place that decides which files count as "the repo". In a
git repo it asks git for tracked plus untracked-but-not-ignored files, so `.gitignore` is
respected for free; otherwise it walks the folder. Either way it then drops
`tool/files.go#defaultIgnore` patterns (dependencies, build output, lock files), binary
extensions, and any extra globs the caller passes. Callers add the docs folder and the config's
`ignore` list where it matters.

`tool/files.go#codeExt` says which extensions are code (for coverage), and
`tool/files.go#refExt` which ones a code ref may use without a slash (package.json).

### Globs

`tool/util.go#globRe` compiles the repo's glob dialect (`*` within a folder, `**` across
folders, `?`, `{a,b}`, trailing `/` for "everything under") to a cached regexp;
`tool/util.go#matchAny` tests a path against a list. Component `paths`, `ignore` lists and the
code panel's file lists all use it. The viewer has a JavaScript copy for static exports
(`tool/assets/index.html#globRe`), checked against this one by
`tool/parity_test.go#TestViewerParity`.

### Git and JSON

`tool/util.go#git` runs git in the repo root and returns trimmed output, with `ok` false if git
fails or isn't installed, so every git feature degrades instead of crashing.
`tool/util.go#writeJSON` writes through a temporary file and a rename, so a crash never leaves
a half-written docs.config.json or manifest.json.

## Interfaces

Called by every command. Calls only git and the file system.

## Watch out for

- `root` is global. Code that needs a path for git or the file system should go through
  `tool/util.go#rel` or `filepath.Join(root, …)`, never the process's current directory.
- The tool uses only Go's standard library ([why](../decisions/0001-go-single-binary.md)). Adding
  a module dependency changes the build and the release workflow.
