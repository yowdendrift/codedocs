# codedocs

Documentation of a codebase, written for the person who has to understand it.

When an AI writes most of the code, you stop holding the shape of the system in your head.
codedocs is a skill for Claude Code (it also works in Codex, Cursor and other agents) that reads a
repository and writes layered docs for a human: an overview first, then the components, the
journeys through the code, the cross-cutting rules, the decisions and the traps. It keeps them
current as the code changes. You read them in a local viewer where every code reference opens the
real file beside the text.

- **Layered.** Overview, then components (structure), flows (behaviour), concerns, decisions,
  glossary and gotchas. Start at the top and go only as deep as you need.
- **Honest.** Anything worked out from the code rather than known is marked *inferred*. What you
  tell it is kept as yours through every rewrite.
- **Checked.** A small tool verifies that every file, symbol and link the docs mention still exists,
  and maps a code change to the pages it affects. It works through git history, so updates are
  cheap.
- **A viewer, not a wiki.** Diagrams (Mermaid) you can zoom, a mind map, search, and a code panel:
  click a reference and the code opens at the right line, imports are links, and Ctrl/Cmd+click
  jumps to a definition.
- **Plain markdown in your repo.** The docs are files in `docs/` that diff well. Commit them or
  don't. Coding agents can read them too.

## Install

### Claude Code

In Claude Code:

```
/plugin install codedocs --marketplace yowdendrift/codedocs
```

On Claude Code older than 2.1.275, add the marketplace first:

```
/plugin marketplace add yowdendrift/codedocs
/plugin install codedocs@codedocs
```

Then open a repository and run `/codedocs init`. If another command already uses that name,
use the full name, `/codedocs:codedocs`.

To get new versions, run `/plugin marketplace update codedocs`, or turn on auto-update for the
marketplace under **Marketplaces** in `/plugin`.

The first command downloads the tool for your machine (one file, about 9 MB) from this
repository's [releases](https://github.com/yowdendrift/codedocs/releases) and checks its
checksum. Nothing else needs to be installed. Git is optional; with it, updates only look at
what changed.

### Codex, Cursor and other agents

The skill uses the [Agent Skills](https://agentskills.io) format, so other agents can load it.
Clone this repository and link (or copy) `skills/codedocs` into your agent's skills folder:

| agent | folder | run it with |
|---|---|---|
| Codex | `~/.agents/skills/codedocs` | `$codedocs init` |
| Cursor | `~/.cursor/skills/codedocs` (Cursor also reads `~/.claude/skills`) | `/codedocs init` |
| Claude Code, by hand | `~/.claude/skills/codedocs` | `/codedocs init` |

```sh
git clone https://github.com/yowdendrift/codedocs
ln -s "$PWD/codedocs/skills/codedocs" ~/.agents/skills/codedocs
```

On Windows: `New-Item -ItemType Junction -Path $HOME\.agents\skills\codedocs -Target <clone>\skills\codedocs`.

It was built and tested in Claude Code. In other agents, it falls back to plain shell commands
where it would use Claude Code features (starting the viewer in the background, splitting a large
repository across subagents). If your agent runs commands in a sandbox without network access,
allow the one-time download on first use, or build the tool yourself (see
[DEVELOPING.md](DEVELOPING.md)).

## Commands

| command | what it does |
|---|---|
| `/codedocs init` | First full pass: an inventory, an outline for you to approve, then the pages. Run it again later to refresh pages written by an older version. |
| `/codedocs add <subject or insight>` | Document a subject in depth (`/codedocs add payments` writes a topic page for admins and/or developers, plus its flows), or record something you know (`/codedocs add "callbacks fail behind Cloudflare unless…"`), kept as yours through every rewrite. |
| `/codedocs update [range]` | Update only the pages affected by changes since the last sync (or a git range). |
| `/codedocs check` | Find and fix drift: missing files or symbols, broken links, code no component covers. |
| `/codedocs explain [range\|branch]` | Explain a change in terms of the docs. Saved under Changes in the viewer. |
| `/codedocs style <description>` | Restyle the viewer from a description ("dark, compact, green accents"). |
| `/codedocs upgrade` | Move a repository's docs to the installed version. |
| `/codedocs links` | Check that the code panel's import links resolve, and add rules for this repository's path aliases if not. |
| `/codedocs serve` | Open the viewer (usually http://localhost:8780/). |
| `/codedocs stop` | Stop the viewer (`stop --all` stops every running one). |
| `/codedocs` or `/codedocs status` | Show where the docs stand. |

The skill only runs when you invoke it, so it never starts an expensive pass by itself. A first
`init` on a mid-sized codebase takes a few minutes and a noticeable share of a session's usage;
`update` afterwards costs a small fraction of that.

## What it adds to your repository

```
docs/                    (or docs/codebase/ if docs/ already holds other files)
  content/               the docs: overview, components/, flows/, concerns/, decisions/,
                         topics/, glossary, gotchas, changes/
  theme.css              your look; /codedocs style edits it; never overwritten
  docs.config.json       project name, editor, ignore globs, last synced commit, version
  manifest.json          generated; drives the viewer
  index.html, viewer.css a static copy of the viewer, for hosting the folder without the tool
```

Served statically (GitHub Pages or any web server), the viewer works without the code panel:
code references open your editor instead, and the libraries load from a CDN.

Adding a short section to your `CLAUDE.md` or `AGENTS.md` points coding agents at the docs:

```
## Architecture docs
Docs for humans and agents live in docs/content. Before changing code, read overview.md and the
page for the component you're touching. After a change, run /codedocs update.
```

## The viewer and security

`codedocs serve` listens on 127.0.0.1 only and rejects requests addressed to any other host. The
code panel is read-only and serves only files the repository itself lists (tracked, or untracked
and not ignored), not binary, and under 2 MB. It never serves `.git`, `node_modules`, `.env*`, or
key and certificate files.

## Platforms

Windows (x64), macOS (Apple silicon and Intel) and Linux (x64 and arm64). The tool is one
executable written in Go with no dependencies outside the standard library, and the viewer's
libraries are built into it.

The release binaries aren't code-signed. Downloaded by the skill they run without prompts; if
you download one from the releases page by hand, macOS may refuse it until you run
`xattr -d com.apple.quarantine <file>`, and Windows SmartScreen may ask you to confirm. Building
from source avoids both.

## Contributing

See [DEVELOPING.md](DEVELOPING.md) for the layout, how to try changes, and how releases work.
