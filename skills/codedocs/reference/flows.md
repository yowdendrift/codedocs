# /codedocs flows

Make (or refresh) the **flow view**: a 3D picture of the code as stacked glass layers (UI, API,
services, data) with each documented flow drawn as a line of light through the functions it
passes. The view also shows the code no flow touches ("gaps"), which is where to document next.

The tool does the mechanical work: listing the code's functions, putting each on a layer,
resolving names, validating. Your job is the judgement: which hops a flow really makes, a short
label for each, which ones touch the database or the outside world, and the branches not taken.
You record that as a `flowgraph` block on each flow page. Read `content-model.md` (the block
format is under "Flow graph blocks") if you haven't this session.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

1. **Scan.** Run `T flows --scan`. It lists:
   - the layers and how many units each holds, and **files on no layer** (put on services for
     now);
   - each flow page and whether it has a block, and which blocks are **stale** (the code they
     name changed since the last `T flows`);
   - route handlers, other entry points (UI events, CLI commands, jobs, queue consumers);
   - **side-effect candidates**: lines that look like database reads and writes, outside calls,
     disk writes, queues and caches, credited to the function they're in;
   - **entry points no flow starts at or runs**: routes, CLI commands and event listeners whose
     handler no step names and no flow calls. Each is a flow still to write (or part of one);
   - units nothing names or reaches, by district, with their names: the real gaps. Units a flow
     calls without naming are counted as *reached* (the view dims them) and left out.
   No flow pages yet: stop and suggest `/codedocs init` or `/codedocs add <subject>`; the flow
   view draws documented flows only.
2. **Layers.** If the scan put files on the wrong layer or on none, write `.codedocs/layers.yml`
   at the repo root: each key is a layer id, top layer first, holding globs. The default layers
   are `ui`, `api`, `svc`, `data`; use your own ids only if the code has a genuinely different
   shape (a game: `scenes`, `systems`, `data`). `ignore` leaves code out of the graph entirely.
   ```yaml
   ui:
     - src/components/**
     - src/pages/**
   api: [src/routes/**, src/middleware/**]
   svc:
     - src/services/**
   data:
     - src/db/**
   ignore:
     - scripts/**
   ```
   A component that sits wholly on one layer can say so instead, with `layer: svc` in its
   frontmatter. The file wins over the component; heuristics (folder and file names) fill the rest.
3. **Write a block on each flow page**, under a `## Flow graph` heading after "Step by step".
   Trace the flow in the code: open only the functions its steps name and follow the calls
   between them. Each line is one hop, in the order things happen:
   ```flowgraph
   `src/ui/SignupForm.tsx#SignupForm` | Form submitted
   branch | Inline errors
   route POST /signup | POST /signup
   `src/services/users.ts#UserService.create` | Create user
   store users | Insert user | db-write
   external Mail provider | Send welcome email
   ```
   - **Units.** A code ref (`` `file#Symbol` ``, the same refs pages use), or
     `route METHOD /path` (a route handler from the scan), `store <name>` (a table or
     collection; `store postgres/users` groups it under `postgres`), `queue <name>`,
     `cache <name>`, `file <name>` (a file or bucket the code writes), `external <name>`
     (anything outside the repo: a payment provider, an email service).
   - **Labels** are what happens, in 2–5 words a newcomer understands ("Create user", not
     "UserService.create"). The view shows them beside the line.
   - **Effects** (third column): `db-read`, `db-write`, `external`, `disk-write`, `queue`,
     `cache`. Tag the step where the effect happens, and only effects you have confirmed in the
     code; the scan's candidates are leads, not facts. `external` units are tagged for you.
   - **Branches**: `branch | label` right after the step they leave from, for paths the
     flow doesn't take: validation errors, a 409, a retry. Add `| flow-id` when the branch is
     another documented flow.
   - **Granularity**: name the function doing the work, not every helper it calls. Five to
     twelve hops is typical. A unit can appear more than once (a service called twice).
   - **Header lines** (optional, before the steps): `id:` (defaults to the page's file name),
     `name:` (defaults to the title), `color: #rrggbb` (defaults to a palette colour).
4. **Cover the repo.** The goal is a flow for every way into the code, not every possible path
   through it (branches multiply; one flow per entry point or user action is the useful
   version). Go down the scan's *entry points no flow starts at or runs* and write a flow page
   (with a block) for each, or fold it into an existing flow when it's the same journey.
   Group trivial ones (static asset routes, a dialog closing) under the flow that uses them,
   as prose or a branch. Don't add helper hops just to raise coverage: reach already counts
   what the steps call, and a flow of 30 hops is unreadable. When code is left that nothing
   names, reaches or could start from, tell the user it may be dead code rather than
   documenting it.
5. **Build it.** `T flows`. It resolves every hop, validates the graph and writes
   `flowgraph.json` (plus `flowgraph.js` and the viewer files) into the docs folder. If it
   reports problems, fix the block lines it points to and run it again; it writes nothing until
   every hop resolves:
   - *no such file* / *no X in file*: the code moved or was renamed; find it and fix the ref.
   - *no route … found*: check the method and path against the scan's route list, or name the
     handler with a code ref instead (also the fix for a route *defined N times*).
   - *another flow already has the id*: two pages share a file name; add `id:` to one block.
   - *unclassified files* are a warning: add them to `.codedocs/layers.yml`.
   More than 2000 units switches the graph to whole files (`--granularity symbol` forces
   functions); refs still work, each hop lands on its file.
6. **Check the result.** `T manifest`, then `T check`, which also checks the blocks' code refs.
7. **Report** in a few lines: flows drawn, coverage (units on a flow / all, and on or reached
   from one, as `T flows` prints them), entry points still without a flow, the biggest gaps by
   district, and how to open it: the **Flow view** entry in the viewer's sidebar (`/codedocs
   serve`), or `flowview.html` in the docs folder, which also works opened as a file.

## Refreshing (from `/codedocs update`)

`T flows --scan` marks blocks as stale when the code they name changed since the graph was last
written. Re-trace only those flows: check each hop still happens in that order, fix refs and
effects, then `T flows`. Don't touch blocks that aren't stale.
