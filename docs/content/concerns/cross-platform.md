---
title: Working on every platform
summary: Windows, macOS and Linux each break something different: locked binaries, line endings, executable bits, shells. How codedocs handles each.
order: 40
components: [distribution, server, cli, docs-model]
generated_with: 0.1.0
updated: 2026-10-29
---
# Working on every platform

## The pattern

One static Go binary per platform does the work, and everything around it is written for the
lowest common denominator:

- **Paths** are forward-slash and repo-relative everywhere inside the tool
  (`tool/util.go#posix`, `tool/util.go#rel`), converted only at the file system boundary.
- **Line endings and BOMs** are normalised when files are read for display or parsing
  (`tool/serve.go#cmdServe`, `tool/cmds.go#jsonUnmarshal`), and `.gitattributes` forces LF on
  shell scripts, Go files and workflows so a Windows checkout doesn't break bash.
- **Shells**: Claude Code runs commands through bash, including Git Bash on Windows, so the
  launcher is POSIX sh; `skills/codedocs/bin/codedocs.ps1` covers Windows without bash.
- **Running binaries are locked on Windows.** `codedocs stop` waits for the server to exit, and
  both build scripts stop running viewers before replacing the binary.
- **Executable bits** don't survive Windows git. The launcher's mode is set in git explicitly and
  CI fails if it's lost; scripts in workflows are run through `sh` so their mode doesn't matter.
- **Opening a browser** uses the platform's own opener (`tool/serve.go#openBrowser`).

## The rule

Test path handling with a Windows path in mind, keep scripts LF and POSIX, and never assume a
file you just built can be overwritten while a viewer is running.

## Where it applies

[Distribution](../components/distribution.md), [viewer server](../components/server.md),
[CLI helpers](../components/cli.md), [docs model](../components/docs-model.md).

## Exceptions

Windows on ARM uses the x64 binary under emulation; there's no native build.
