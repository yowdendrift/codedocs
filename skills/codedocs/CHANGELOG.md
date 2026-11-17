# Changelog

Each entry says what changed and whether it affects existing docs:
- **viewer**: picked up by `/codedocs upgrade` (index.html / viewer.css replaced; theme.css kept)
- **content model**: folders, frontmatter, ref or link formats; `/codedocs upgrade` applies these mechanically
- **guidance**: how pages are written. Also bump `reference/VERSION` (the page-guidance version
  pages are stamped with); pages stamped with an older one show as "outdated", and
  `/codedocs init` → "Refresh outdated pages" rewrites just those. Tool or viewer releases
  leave it alone, so they never mark pages outdated.
- **theme**: new variables in the default theme.css; `/codedocs upgrade` adds them to existing themes

## 0.7.1 (2026-11-17)
- **viewer, tool: reached code isn't a gap.** A flow names the hops that matter, not every
  helper, so the flow view counted helpers under documented steps as undocumented.
  `codedocs flows` now follows calls out of each flow's steps (by name, so approximate) and
  marks the units a flow runs without naming as *reached*; they show dimmed rather than red,
  the coverage pill adds "· N% reached", and a reached unit's card names its caller and flow.
  The walk stops at entry handlers (what a CLI command or event listener hands off to), which
  start flows of their own. `flowgraph.json` units may carry `reached: {flow, via}`
  (optional; the schema stays version 1).
- **viewer**: the flow view picks flows from a dropdown at the top instead of a row of buttons
  along the bottom, which ran off the screen once a project had more than a handful of flows.
- **viewer, tool**: function spans no longer stop at 400 lines (now 1000), and a TypeScript
  function whose return type is an object literal (`): { dispose(): void } {`) no longer ends
  on its first line. Both cut code panel highlights and flow code views short.
- **tool**: `flows --scan` lists the **entry points no flow starts at or runs** (the flows still
  to write) and names the units nothing names or reaches.
- **guidance**: `/codedocs flows` covers the repo: a flow per uncovered entry point, no helper
  hops added just for coverage, and unreached code reported as possible dead code. Additive, so
  the page-guidance version is unchanged.

## 0.7.0 (2026-11-17)
- **viewer: the flow view.** A 3D picture of the code as stacked glass layers (UI, API,
  services, data, or the repo's own), each documented flow drawn as a line of light through the
  functions it passes, with side effects in coral and branches not taken as dashed stubs. Modes:
  a light-up tour on first visit, one flow (step by step, with a caption), and gaps (code on no
  documented flow, in red, with the coverage figure). There is no "all flows" mode: nothing runs
  until a flow is picked, and picking it again lets it go. Click a unit for a card that opens
  its code in the code panel. **2D view** lifts the selected flow out of the layers into a flat,
  straight column, one row per unit in step order, with each step's label beside it and a
  divider where it changes layer; the units fly between the two views, the layers fade, and the
  camera faces it square on (pan and zoom only). Camera presets
  3D / Side / Top; deep links `#flow-<id>`, `#focus-<id>` and `#gaps`; touch, reduced motion,
  and an SVG side view where WebGL isn't available. It opens from
  a **Flow view** entry in the sidebar, from a flow page, or as `flowview.html` in the docs
  folder, which works opened straight from disk. Static exports include it.
- **`codedocs flows`** builds the graph behind it (`flowgraph.json`, schema in
  `schema/flowgraph.v1.json`) from `flowgraph` blocks on flow pages, the repo's functions and
  route handlers, and layers from `.codedocs/layers.yml`, a component's `layer`, or folder and
  file names. It validates before writing and writes nothing if a hop doesn't resolve.
  `flows --scan` lists layers, unclassified files, routes and other entry points, side-effect
  candidates, stale blocks and uncovered code, for the agent. `serve` and `export` build the
  graph themselves; `check` checks the blocks.
- **content model**: flow pages may carry a ```` ```flowgraph ```` block (one hop per line:
  `unit | label | effect`, `branch | label`), shown in the docs viewer as a numbered list.
  Components may set `layer`. Both optional; existing docs are unaffected.
- **guidance**: new `/codedocs flows` guide; `update` re-traces stale blocks. Additive, so the
  page-guidance version is unchanged and no pages show as outdated.
- **viewer**: the sidebar's sections open one at a time from their headings; going to a page
  opens its section.
- **viewer**: the mind map now uses the theme's colours. In dark mode its headings ("Components",
  "Flows"…) and the "via …" lines were dark grey on a dark background; markmap's own defaults
  were overriding the theme.
- **viewer: read a flow as code.** Flow pages get a Prose / Code toggle. The Code view shows each
  step's description followed by the full body of every function the step names, in step order:
  one top-to-bottom read across all the files the flow touches. Click an underlined call to
  expand the callee's body in place (and again inside that); Ctrl/Cmd+click any name still opens
  its definition in the panel. Each block shows the other pages that name the same symbol, and
  an approximate call count ("≈7 calls in 3 files") that lists the call sites. Works served and
  in static exports.
- **Symbol extents**: the tool now works out where a definition ends as well as where it starts
  (brace matching that skips strings, comments, template literals and regexes; indentation for
  Python and GDScript). The manifest gains `spans` (`"file#Symbol"` → `[from, to]`, including
  doc comments above) and each page's `syms`. Go-to-definition results and `code/defs.json`
  carry the end line; exports add `code/callers.json`. The server adds `/api/callers`.
- **`check` warns about flow steps that name no function** (`step-without-symbol`), or only a
  long code file. Warnings don't fail the check.
- **guidance**: flow steps should name the function doing the work as `file#Symbol`.
  (`reference/VERSION` not bumped: existing pages mostly comply, and `check` points at the rest.)

## 0.6.0 (2026-10-29)
- **`codedocs export <folder>`** (new): writes the docs as a static site for GitHub Pages or any
  web host, with a working code panel. The source isn't copied: the viewer fetches each file from
  `raw.githubusercontent.com` at the commit the docs describe (`last_synced`, else `HEAD`), and
  `code/` holds the file list, import links and a definitions index. Pass `--github OWNER/REPO`
  and `--ref` to override, or `--source-url`/`--web-url` for another host. In Claude Code:
  `/codedocs export`.
- **viewer**: a static mode for exports. Clicking code refs, following imports, file trees and
  Ctrl/Cmd+click all work without the server; "Open in editor" becomes "View on GitHub". Served
  by `codedocs serve`, nothing changes.
- **Symbol refs resolve more accurately**: a declaration keyword now has to stand on its own, so
  HTML like `class="…"` or a name like `constructor` no longer counts as a definition. A few refs
  that used to land on the wrong line now land on the right one.
- A saved default theme now lives in your config folder (`codedocs/default-theme.css`), so plugin
  updates don't lose it. One saved in the skill folder by an older version is still read.
- This repository now documents itself in `docs/`, published as a live demo on every push.

## 0.5.0 (2026-10-29)
Packaging release: no changes to the viewer, the content model, the guidance or themes, so
existing docs need nothing.
- **Install as a Claude Code plugin.** The repository is now a plugin marketplace:
  `/plugin install codedocs --marketplace <owner>/codedocs`. The skill moved from `skill/` to
  `skills/codedocs/`; re-run `install.ps1` if you linked it by hand.
- **Binaries aren't in git any more.** A tagged release builds them (GitHub Actions) and
  publishes them with a `SHA256SUMS` file. On first run the launcher downloads the one for the
  machine, for the version in `VERSION`, checks it, and keeps it in `bin/`. A local build still
  takes precedence. `CODEDOCS_DOWNLOAD_BASE` points it at a mirror.
- **`bin/codedocs.ps1`**: a PowerShell launcher for Windows without Git Bash.
- **Works in other agents.** `SKILL.md` no longer depends on Claude Code features to make
  sense: paths are written as `<skill>/...` and defined once, starting the viewer and
  splitting work across subagents are described for agents without those tools, and
  `agents/openai.yaml` keeps Codex from starting it unasked.

## 0.4.2 (2026-10-29)
- **viewer**: on phones and narrower windows (under 1180px) the "On this page" rail is now a
  drawer from the right, opened with a button in the top-right corner. It closes when you pick
  a link or tap outside it.
- **theme**: the default `theme.css` no longer contains a commented `--page-max-width` example
  (an upgrade could copy it in and cap the page width), and `upgrade.md` now says never to copy
  values out of comments or add optional caps.

## 0.4.1 (2026-10-29)
- **viewer**: click any diagram (or its expand button) to open it full screen. Scroll to zoom
  around the pointer, drag to move, double-click or 0 to fit, +/- to zoom, Esc to close. Large
  diagrams open at a readable size from their start; "fit" shows the whole thing. Text stays
  sharp at any zoom.
- **viewer**: the page now fills the space between the sidebar and the rail (themes can cap it
  with `--page-max-width`; the old `--content-width` is no longer used).
- **viewer**: the rail card always runs the full height of the window.

## 0.4.0 (2026-10-29)
- **`/codedocs add <subject | insight>`** (new `reference/add.md`):
  - a **subject** ("payments"): finds the code with `codedocs find`, checks what the docs
    already cover, asks who it's for (admins, developers or both) and proposes an outline, then
    writes a topic page plus any new flows, gotchas, glossary terms and decisions, and links it
    from the overview;
  - an **insight** (something you know that the code doesn't show): recorded where a reader
    needs it, as a "From the team" callout, a stated decision, a gotcha or a glossary term,
    and reconciled with anything the docs inferred.
- **content model**: new `topics/` page kind (`audience: admin | developer | both`,
  `components: [...]`) and the `> [!INSIGHT]` callout. No changes to existing pages needed.
- **guidance**: "Writing for admins" in `writing.md`; every rewrite (`init`, `update`) must carry
  insight callouts and stated decisions over unchanged.
- **viewer**: Topics group in the sidebar and on the map, audience badge on topic pages,
  "From the team" callouts, and component pages list the topics that involve them.
- **theme**: optional `--kind-topic` and `--insight` colours.
- `codedocs find <terms…>`: where a subject lives (folders, files and definitions by name),
  without reading the code.

## 0.3.4 (2026-10-29)
- **viewer**: the right-hand rail is now a card of its own (sidebar background, border, rounded
  corners) that sits closer to the window edge and fits within the window, so its scrollbar
  reads as part of it instead of floating next to the page's scrollbar.
- **viewer**: file trees use a wider indent per level, so the folder guide lines aren't crowded.
- **theme**: optional `--rail-bg` and `--rail-border` (default to the sidebar background and border).

## 0.3.3 (2026-10-29)
- **viewer**: the right-hand rail ("On this page") is 50% wider on large screens (up to 22.5rem,
  scaling down with the window so the page text keeps its room).
- **viewer**: the rail's Code section is now a file tree. When served, it shows the component's
  actual files; click one to open it in the code panel. Folder chains with nothing else in them
  are merged (`addons/Huddle/Matchday/`) and each level is only slightly indented, to keep it
  narrow. Files mentioned in the docs get a dot; components with more than 30 files start with
  their subfolders folded. Flows and other pages get a "Code mentioned" tree of the files they
  reference. Statically hosted docs show the path patterns as a tree instead.

## 0.3.2 (2026-10-29)
- **viewer**: fix: when several imported names shared one stretch of text
  (`import { a, b, c } from './x'`), the first link swallowed the rest of the line, so every name
  went to the first one. Each name now gets its own link.

## 0.3.1 (2026-10-29)
- **viewer**: a "Browse the code" button (folder icon) in the code panel's header. It lists the
  files of the component that owns the open file, or of the current page, so you can browse
  without closing the panel (the rail's button is hidden while the panel is open). It joins the
  panel's back history like any other step.

## 0.3.0 (2026-10-29)
- **viewer**: language packs for code links, now resolved by the tool (so only links that
  actually lead somewhere are shown):
  - **JS/TS** (+ Vue, Svelte, Astro): as before, plus `tsconfig.json`/`jsconfig.json` `paths` and
    `baseUrl`, read automatically.
  - **PHP**: `use` statements (including group `use A\{B, C}`), and any qualified name
    (`\XF\Entity\User`, `extends`, `::class`, docblocks). Resolves via `composer.json` PSR-4,
    the namespace path, and path-suffix matching, which covers XenForo's `src/XF`,
    `src/addons/Vendor/Addon` and single-addon repos without configuration.
  - **Go**: imports inside the module (`go.mod`) open the package's file list.
  - **Python**: `import a.b`, `from a.b import c`, relative imports, submodules, `__init__.py`.
  - **Godot**: `res://` paths in GDScript, scenes and resources (`preload`, `load`, `extends`,
    `ext_resource`).
  - **CSS/SCSS/Less**: `@import`, `@use`, `@forward`, including SCSS partials.
- **viewer**: **Ctrl/Cmd+click any name** to go to its definition, in any language. Uses the
  file's own imports first (so `XFUser`, `V2` or `r.Render` land on the right file), then a
  definition index of the whole repo; if a name is defined in several places you get a picker.
  Languages without a pack (Java, C#, C/C++, Rust…) get this too.
- **viewer**: a link whose text spans several highlight spans (PHP namespaces) now works.
- `codedocs links [FILE]`: a per-language summary of resolved and unresolved local references,
  or one file's links. New `/codedocs links` step (also run at the end of `init`) has Claude add
  repo-specific rules to `docs.config.json` (`links.aliases`, `links.roots`) when imports don't
  resolve, for example Vite aliases or a custom PHP autoloader.
- Symbol refs now prefer real declarations (`class Entity`) over other mentions (`namespace …\Entity`).

## 0.2.2 (2026-10-29)
- **viewer**: clickable imports in the code panel (JS/TS family: .ts, .tsx, .js, .jsx, .mjs, .cjs,
  .vue, .svelte). Click an import path to open that file, or an imported name to open the file
  scrolled to where it's defined. Handles multi-line imports, `export … from`, `import()` and
  `require()`, `type` imports, `X as Y`, extensionless and `index` files, `.js` specifiers that
  point at `.ts` sources, and `@/` / `~/` aliases (tried as `src/` and the repo root).
  Package imports (`react`) aren't linked.
- **viewer**: the code panel's back button now steps back through everything you opened in it
  (files, followed imports, file lists), restoring the scroll position.

## 0.2.1 (2026-10-29)
- **viewer**: thin, theme-coloured scrollbars everywhere (sidebar, page, code panel, map).
  Uses the standard `scrollbar-width`/`scrollbar-color` where supported, with a WebKit fallback
  for Safari and older Chromium.
- **theme**: new optional variables `--scrollbar-thumb`, `--scrollbar-thumb-hover` and
  `--scrollbar-track`. Older themes get defaults based on `--muted`.
- `codedocs stop` (`/codedocs stop`, alias `shutdown`) stops this repo's viewer from any session;
  `stop --all` stops every running viewer. A running `serve` registers itself in the temp folder
  and accepts a token-protected shutdown request, so this works the same on every OS without
  killing processes. Running `serve` again for a repo that's already being served reports the
  address instead of starting a second server. Ctrl+C shuts down cleanly.
- `build.ps1` / `build.sh` stop running viewers first, since Windows locks a running `codedocs.exe`.
- `.gitattributes` keeps the launcher's LF line endings; the launcher restores a lost
  executable bit.

## 0.2.0 (2026-10-29)
- **No more Node.** The tool is now a single Go executable, `bin/codedocs` (a launcher that picks
  `codedocs.exe` on Windows, or the macOS/Linux build). Same commands and output as before.
  Build with `build.ps1` / `build.sh` (needs Go 1.22+; no third-party Go modules).
- **viewer**: code panel. When the docs are served by `codedocs serve`, clicking a code ref opens
  the file beside the docs, syntax-highlighted, scrolled to the line or symbol, with the range
  highlighted. Shows which component owns the file and which pages mention it. Component pages
  get "Browse the code" (every file in the component's `paths`); other pages list the files they
  mention. Click a line number to copy a `path:line` ref (Shift+click for a range). Ctrl/Cmd+click
  a ref still opens your editor. The panel is resizable; Esc closes it.
- **viewer**: the viewer and its libraries (marked, Mermaid, Markmap, highlight.js) are built into
  the binary, so `serve` works offline and always uses the installed version. Statically hosted
  docs folders fall back to the CDN, or use `install-viewer --offline` to copy the libraries in.
- **viewer**: `serve` rebuilds the manifest on every page load (edits show on refresh), picks the
  next free port if 8780 is busy, and `--open` opens the browser. It only answers requests to
  localhost and only serves the repo's own source files (not ignored, binary or `.env`/key files).
- **theme**: new variables for the code panel: `--code-hl` and `--syn-keyword`, `--syn-string`,
  `--syn-comment`, `--syn-number`, `--syn-function`, `--syn-type`, `--syn-attr`, `--syn-meta`.
  Older themes get sensible defaults, so adding them is optional.
- Personal default theme moves to `default-theme.css` in the skill folder.
- Pages are now stamped with the page-guidance version (`reference/VERSION`, currently 0.1.0)
  rather than the tool version, so tool releases don't mark every page as outdated.
- No content model or guidance changes; existing docs need only `/codedocs upgrade`.

## 0.1.1 (2026-10-29)
- Renamed the skill from `docs` to `codedocs`: `/docs` clashed with the built-in docs skill
  (Claude Docs documents). Commands are now `/codedocs init`, `/codedocs update`, …
  Reinstall with `install.ps1`; it removes the old `docs` link.
- `install-viewer` (tool) defaults to `docs/codebase/` when `docs/` already holds other files.
- No content model, guidance or theme changes; existing docs folders need nothing.

## 0.1.0 (2026-10-28)
- First version: `init`, `update`, `check`, `explain`, `style`, `upgrade`, `serve`, `status`.
- Content model: overview, components (with `paths`), flows, concerns, decisions, glossary,
  gotchas, changes. Cross-links declared once via `components: [...]`.
- Code refs: `path`, `path#Symbol`, `path:line`. Symbols resolved to lines by the tool.
- Viewer: sidebar by kind, related-pages rail, Mermaid diagrams, Markmap mind map, full-text
  search, editor links (VS Code / Cursor), light/dark/auto.
