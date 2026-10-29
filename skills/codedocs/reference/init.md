# /codedocs init

First full documentation pass, or a re-run when docs already exist. Read `content-model.md` and
`writing.md` (same folder) before writing any page.

`T` below means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

## 1. Existing docs? Pick a mode

Run `T status`.

- **No docs folder:** go to step 2.
- **Docs exist:** tell the user what `status` reports (versions, page counts, how far behind HEAD,
  pages written with older page guidance), then ask which mode they want:
  1. **Refresh outdated pages:** rewrite only pages whose `generated_with` is older than the
     current page guidance (`T version --guidance`); the usual choice after improving the guides.
  2. **Regenerate everything:** a full fresh pass. Keeps `theme.css`, `docs.config.json`,
     locked pages, `status: stated` decisions and `> [!INSIGHT]` blocks (carried into the
     rewritten pages); everything else is rewritten. Topic pages are rewritten too.
  3. **Selected pages:** the user names pages or kinds ("all flows", "the overview").
  4. **Viewer only:** same as `/codedocs upgrade`.

  In modes 1–3, run `T install-viewer` first (to pick up viewer fixes), skip step 3's outline
  approval unless the component list changes, and write only the chosen pages in step 4.

## 2. Inventory (cheap)

1. Run `T stats`. It lists directories with file and line counts, extensions, manifests, the
   largest and most-changed files. This is your map; don't list or count files yourself.
2. Read the README, the main manifest(s) (package.json, composer.json, go.mod, project.godot…),
   and the entry points. Stop there for now.
3. Ask the user, in one message, anything you can't tell from that:
   - the project's name and a sentence on what it's for, if the README doesn't say;
   - where the docs should live. The tool defaults to `docs/`, or `docs/codebase/` when `docs/`
     already holds other files (plans, guides): say which it will use and let them override;
   - which editor to link code refs to: `vscode` (default), `cursor`, or `none`;
   - anything they already know is important or confusing ("the undo system is a mess").

## 3. Outline (get it right before writing)

Work out the components, flows and concerns:

- **Small repo** (under ~150 source files or ~20k lines): read what you need yourself, following
  imports from the entry points.
- **Larger repo:** split the work. Spawn one `Explore` subagent per candidate area, in parallel,
  each with a tight brief: *"Read <dirs>. Report in under 300 words: what this area is
  responsible for, its main parts (with file#symbol refs), what it calls and what calls it, state
  it owns, anything surprising. Don't paste code."* You keep only their summaries, so your context
  stays free for writing. (In Claude Code that's the `Explore` agent; elsewhere use your
  read-only subagent. With no subagents, take the areas one at a time and keep only notes of
  that shape.)

Then present the outline to the user and **wait for confirmation** before writing pages:

```
Components (n):  <slug>: <one line>   paths: <globs>
Flows (n):       <slug>: <one line>   via: <components>
Concerns (n):    <slug>: <one line>
Decisions:       <titles> (stated/inferred)
Glossary:        ~N terms     Gotchas: ~N so far
Coverage:        <dirs not claimed by any component, and what you plan for them>
```

This is the cheapest moment for the user to correct you ("those two are one thing", "you missed
the audio sync"). Apply their corrections.

## 4. Write

1. `T install-viewer --project "<name>"` (add the folder name, e.g. `docs/codebase`, as a
   positional argument if not using `docs/`). Then set `editor` and any `ignore` globs in
   `docs.config.json`.
2. Write pages in this order, so each builds on the last:
   1. components (structure first; you need their slugs for everything else),
   2. flows (trace each through the real code; this is where you read the most),
   3. concerns,
   4. decisions and gotchas (collect them as you go; write them now),
   5. glossary (terms you used),
   6. overview (last, when you know the whole shape).
3. For larger repos, you may hand page *drafting* to subagents (one per component, with the
   outline, the content model and the writing guide as their brief) and then edit their drafts
   for consistency. Never let two writers produce the same page.
4. Use component slugs consistently in `components: [...]` lists. Give every component `paths`.

## 5. Check and finish

1. `T manifest`, then `T check`. Fix every reported issue: wrong path, symbol renamed, broken
   link. For coverage gaps, either extend a component's `paths`, add `ignore` globs for
   generated or vendored code, or tell the user what's undocumented and why.
2. Code-panel links: follow `<skill>/reference/links.md` (run `T links`; add
   `links` rules to `docs.config.json` only if local imports don't resolve). It's quick.
3. `T mark-synced` (records HEAD as the commit the docs describe; skip if not a git repo).
4. Report in a few lines: pages written by kind, what you were unsure about (inferred decisions,
   areas you understood least), and how to view: `/codedocs serve`.
5. Offer (don't do unasked) to add a short section to the repo's `CLAUDE.md` (or `AGENTS.md`, whichever it uses) so coding agents use
   the docs too:
   ```
   ## Architecture docs
   Docs for humans and agents live in <docs folder>/content. Before changing code, read
   overview.md and the page for the component you're touching. After a change, run /codedocs update.
   ```
6. Suggest the real test: pick a small change and make it by hand using only the docs. Where they
   get stuck is where the docs need work.

Don't commit anything. The user decides whether the docs go in git.
