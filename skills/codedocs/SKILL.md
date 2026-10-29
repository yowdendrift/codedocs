---
name: codedocs
description: Codebase documentation for humans (not Claude Docs documents). Documents a codebase so a person can understand it, as layered markdown docs (overview, components, flows, concerns, decisions, glossary, gotchas) read through an HTML viewer with diagrams, a mind map and a code panel. Also keeps the docs current after changes, checks them for drift, explains a change in terms of the docs, and restyles the viewer.
argument-hint: "init | add <subject|insight> | update [range] | check | explain [range|branch] | style <description> | upgrade | links | serve | stop | export [folder] | status"
disable-model-invocation: true
allowed-tools:
  - Bash("${CLAUDE_SKILL_DIR}/bin/codedocs" *)
  - Bash("${CLAUDE_SKILL_DIR}/bin/codedocs.exe" *)
---

# /codedocs

You are documenting a codebase **for a human who has to understand it**: someone who did not
write it (often because an AI did) and needs a working mental model of how it fits together.
The docs are also read by AI agents as context, so they must be accurate and cheap to read.

Arguments: `$ARGUMENTS` (if that reads literally, the arguments are what the user wrote after the
skill name)

**Paths.** `<skill>` in this file and in the guides means this skill's folder:
`${CLAUDE_SKILL_DIR}`. (If that path still reads as a `${...}` variable, your agent didn't fill it
in: `<skill>` is the folder this SKILL.md is in.) Write it out in full in commands, quoted.

## Dispatch

Take the first word of the arguments and follow the matching guide. Read only that guide,
plus the shared references it tells you to read.

| command | guide | what it does |
|---|---|---|
| `init` | `<skill>/reference/init.md` | First full documentation pass, or a re-run after the skill has improved |
| `update [range]` | `<skill>/reference/update.md` | Bring the docs in line with code changes since the last sync |
| `check` | `<skill>/reference/check.md` | Find and fix drift: broken code refs, links, uncovered code |
| `explain [range\|branch]` | `<skill>/reference/explain.md` | Explain a change in terms of the existing docs |
| `style <description>` | `<skill>/reference/style.md` | Change how the docs look |
| `upgrade` | `<skill>/reference/upgrade.md` | Move a repo's docs to the installed skill version |
| `links` | `<skill>/reference/links.md` | Make the code panel's import links work for this repo |
| `add <subject \| insight>` | `<skill>/reference/add.md` | Document a chosen subject in depth (a topic page plus flows), or record something the user knows |
| `serve` | (below) | Start the viewer |
| `stop` (or `shutdown`) | (below) | Stop the viewer |
| `export [folder]` | (below) | Write the docs as a static site for GitHub Pages or any web host |
| `status` or nothing | (below) | Show the state of the docs and the command list |

**serve**: run `"<skill>/bin/codedocs" serve --open` from the repo root so that it keeps running
without blocking you: in Claude Code with `run_in_background`; elsewhere as a background process
(bash: add `> /dev/null 2>&1 &`, then run `serve` again normally, which reports the running
viewer's address and exits). It opens the browser and prints the address (usually
`http://localhost:8780/`; it picks the next free port if that one is busy). Tell the user the
address, and that clicking a code reference opens the code beside the docs (Ctrl/Cmd+click opens
their editor), clicking an import opens that file, and Ctrl/Cmd+click on a name jumps to its
definition. Add `--port N` if they ask for a different port. If the viewer for this repo is
already running, `serve` says so and exits instead of starting a second one: just give the user
the address.

**stop** (also `shutdown`): run `"<skill>/bin/codedocs" stop`, or `stop --all` if the
user wants every running viewer stopped (for example before rebuilding the binary on Windows,
where a running copy is locked). It works whichever session started the server. Report what it
printed.

**export**: run `"<skill>/bin/codedocs" export <folder>` from the repo root (default folder
`docs-site`; the tool refuses a non-empty folder that isn't an earlier export). It writes a static
copy of the viewer and docs whose code panel loads source from GitHub at the commit the docs
describe, so it needs the repo on GitHub and that commit pushed; the tool warns if it isn't.
Report what it printed, and that any static host works: GitHub Pages, Netlify, a plain web
server. Suggest adding the export folder to `.gitignore` unless they mean to commit it. For a
private repository, tell them visitors won't be able to load the code.

**status**: run `"<skill>/bin/codedocs" status`. If it reports no docs
folder, say so and suggest `/codedocs init`. Then list the commands in the table above in one line each.

## The tool

`"<skill>/bin/codedocs" <command>` is a single executable (no Node or other runtime
needed) that does all the deterministic work:
`stats`, `install-viewer`, `manifest`, `check`, `affected`, `mark-synced`, `status`, `links`, `find`,
`serve`, `stop`, `export`, `version`. Always quote the path (it may contain spaces or backslashes). Run it
from the repo root. It finds the docs folder itself; pass `--dir <folder>` only if it can't.

`bin/codedocs` is a small launcher script. The first time it runs on a machine it downloads the
right binary (about 9 MB) from the project's GitHub release for this version, checks it and keeps
it in `<skill>/bin`; it says so on stderr, which is expected. Where bash isn't available (Windows
PowerShell), use `& "<skill>\bin\codedocs.ps1" <command>` instead, which does the same. Once
downloaded, `"<skill>\bin\codedocs.exe"` can also be called directly. If the download fails,
show the user the message it printed and stop.

## Rules for every command

1. **Spend tokens on understanding, not on bookkeeping.** Anything the tool can do (listing files,
   counting lines, finding which pages a change touches, checking refs) you do with the tool, never
   by reading files yourself. Read source code selectively: entry points, the files a flow passes
   through, the files a change touches. Never read the whole repo into your context.
2. **The docs explain what the code can't.** Purpose, shape, how things connect, why, and the traps.
   Never restate code line by line. Follow `<skill>/reference/writing.md`.
3. **One content model.** Every page follows `<skill>/reference/content-model.md`:
   folders, frontmatter, code-ref format, link format, callouts and diagrams. The viewer and the
   tool both depend on it.
4. **Mark what you inferred.** Anything you worked out from the code rather than being told by the
   user (especially *why* something was done) is marked as inferred. Never present a guess as fact.
5. **Stamp what you write.** Set `generated_with` in the frontmatter of every page you create or
   rewrite to the output of `"<skill>/bin/codedocs" version --guidance` (the version
   of these writing guides, not of the tool), and
   `updated` to today's date.
6. **Respect `locked: true`.** Never rewrite a locked page. If it looks wrong, say so and suggest the change.
   **Keep the user's own knowledge.** `> [!INSIGHT]` blocks and `status: stated` decisions record
   what the user told you. When you rewrite or update any page, carry them over unchanged (fix
   only their code refs if those moved).
7. **Never edit `index.html` or `viewer.css` in a repo's docs folder.** They are built into the
   codedocs binary (which serves its own copy) and replaced on upgrade. Look and feel changes go in `theme.css` (via `/codedocs style`).
8. **Finish with the tool.** After changing any content, run `manifest` then `check`, and fix what
   `check` reports before you finish.
9. **Be brief in chat.** The docs are the output. At the end, say what changed in a few lines and
   how to view it.
