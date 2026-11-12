---
title: Viewer
summary: The single-page web app people read the docs in: navigation, markdown with diagrams, a mind map, search, and the code panel. One HTML file plus a stylesheet and a theme.
order: 70
paths: [tool/assets/**, tool/vendor.sh]
generated_with: 0.1.0
updated: 2026-11-12
---
# Viewer

## Responsibility

Everything a reader sees. It loads manifest.json, renders pages from `content/`, draws
diagrams and the map, searches, and runs the code panel. It doesn't know where it's hosted: the
[server](server.md), a static web host, or an [export](export.md) with source on GitHub all look
the same to it, apart from where code comes from.

## How it works

`tool/assets/index.html` is the whole app: markup, and one script in an immediately-invoked
function with no build step, no framework and no bundler. It's embedded into the binary at build
time. Libraries load on demand from `lib/` with a CDN fallback (`LIBS`): marked for markdown,
Mermaid for diagrams, D3 and markmap for the map, highlight.js for code. Routing is the URL hash
(`#/components/server`, `#/map`, `#/search/…`, `#/as-code/flows/…`), so any view can be linked
to and the back button works.

Looks are split in two. `tool/assets/viewer.css` is layout and is replaced on upgrade;
`tool/assets/theme.css` is the default set of CSS variables (fonts, colours, the colour of each
page kind) that a repo's own theme.css overrides and `/codedocs style` edits. Layout only ever
reads variables, so a theme can't break the layout and an upgrade never overwrites a theme.

## Parts

### Pages and markdown

`tool/assets/index.html#init` loads the manifest, detects whether a code panel is available,
builds the sidebar (`tool/assets/index.html#buildNav`) and routes. `tool/assets/index.html#showPage`
renders a page with marked, using custom renderers set in `tool/assets/index.html#setupMarked`:
inline code that matches a code ref becomes a link (`tool/assets/index.html#refHtml`), callouts
like `[!INFERRED]` and `[!INSIGHT]` get their styles, and `(inferred)` in prose is tagged
(`tool/assets/index.html#tagInferred`). The right-hand rail (`tool/assets/index.html#rail`) shows
the page outline, related pages and, for components, a file tree.

### Diagrams and the map

`tool/assets/index.html#renderDiagrams` turns mermaid blocks into SVG, themed to match light or
dark mode; clicking one opens a zoomable, draggable full-screen view
(`tool/assets/index.html#openDiagram`). `tool/assets/index.html#showMap` builds a markmap
mind map from the manifest: components and their parts, flows, concerns.

### Search

`tool/assets/index.html#showSearch` fetches every page once, scores title, heading and body
matches, and shows highlighted snippets. Everything runs in the browser.

### Code panel

`tool/assets/index.html#showCode` opens a file beside the page at a line or symbol,
highlights it (`tool/assets/index.html#highlight` then `tool/assets/index.html#splitHtmlLines`
to keep spans intact per line), turns imports into links (`tool/assets/index.html#applyLinks`),
and shows which component owns the file and which pages mention it. Ctrl/Cmd+click on a name
calls `tool/assets/index.html#goToDefinition`. A small history stack makes the panel's back
button work across files, file lists and definition choices.

### Flows as code

Flow pages get a Prose / Code toggle. The Code view (`tool/assets/index.html#showSlice`) takes
the steps from the page's markdown (`tool/assets/index.html#flowSteps`: the `###` headings under
"Step by step") and, under each step's text, shows the body of every function the step names.
The lines to show come from the manifest's `spans` (`tool/assets/index.html#refRange`); a
whole-file ref shows the first 40 lines with a button for the rest. Each block is filled by
`tool/assets/index.html#fillBlock`. It also gets an "Also in" list of other pages that name the
same symbol (`tool/assets/index.html#alsoIn`, from each page's `syms`) and an "≈N calls" button
that lists the call sites from `Code.callers`. `tool/assets/index.html#markCalls` underlines
anything that looks like a call. Clicking one runs `tool/assets/index.html#expandCall`, which
looks up the definition and inserts the callee's body under that line, so a reader can follow
the code down without leaving the page. Several matches, or a body over 400 lines, open in the
code panel instead.

### Where code comes from

`tool/assets/index.html#Code` hides the difference between serving and a static export. With the
server it calls `/api/file`, `/api/files`, `/api/definition` and `/api/callers`. In an export
(the manifest says `static`), it reads code/index.json, fetches source from GitHub, filters file
lists with its own `tool/assets/index.html#globRe`, ranks definitions from code/defs.json the way
the server would, and reads call sites from code/callers.json. "Open in editor" becomes "View on
GitHub".

### Libraries

`tool/assets/lib/` holds the libraries gzipped; `tool/assets/lib/VERSIONS.txt` lists versions
and licences, and `tool/vendor.sh` refreshes them from npm. The CDN URLs in `LIBS` must match
those versions.

## Interfaces

Reads manifest.json (from the [docs model](docs-model.md)), `content/*.md`, theme.css, and
either the server's `/api/*` or an export's `code/`. Writes nothing except a theme choice in
`localStorage`.

## Watch out for

- Never edit index.html or viewer.css in a repo's docs folder: `serve` ignores them and
  `/codedocs upgrade` replaces them. Change `tool/assets/` and rebuild.
- Opened as a `file://` page, the viewer can't fetch anything and says so. It must be served.
- `tool/assets/index.html#flowSteps` and `tool/cmds.go#flowStepGaps` (behind `check`'s
  `step-without-symbol` warning) both decide what counts as a step. Change one, change both, or
  `check` will warn about steps the Code view doesn't show, or miss gaps it does.
- In an export, call counts exist only for symbols the docs name: code/callers.json is
  precomputed for those, so a function opened by expanding a call gets no count.
- Every function lives inside one closure, so nothing is reachable from the browser console.
  Debugging means temporarily exposing what you need.
