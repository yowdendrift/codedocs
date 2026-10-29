# /codedocs add <subject | insight>

Grow the docs by hand-picked additions: a **subject** to research and document
("payments", "how permissions are cached"), or an **insight** the user knows that the code
doesn't show ("payment callbacks fail behind Cloudflare unless the provider's IPs are
allowed"). Read `content-model.md` and `writing.md` (same folder) first.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

## 0. Which kind?

- A noun phrase ("payments", "the alert system") → **subject** (section A).
- A statement of fact, advice, history or intent ("we never …", "X breaks when …", "this was
  done because …") → **insight** (section B).
- Both ("payments: refunds are handled manually") → do A, and fold the statement in as B.
- Unclear: ask in one line.

If there are no docs yet (`T status`), suggest `/codedocs init` first and stop.

## A. A subject

1. **Check what exists.** Search the docs for the subject (Grep `content/`), including topics,
   components, flows, glossary and gotchas. Note what's already covered so you extend rather
   than duplicate.
2. **Find the code, cheaply.** Pick 2–5 search terms: the subject, its synonyms and the names
   the code is likely to use (for payments: `payment purchase upgrade paypal stripe callback`).
   Run `T find <terms…>`. It lists the folders and files that mention them and the definitions
   whose names match. Don't Grep the repo yourself for this.
3. **Read only what matters.** Entry points and the files at the top of `find`'s list. For a
   large subject (more than ~15 relevant files), use parallel read-only subagents per area,
   as in `init`, and keep only their summaries.
4. **Ask who it's for, and propose an outline.** Audience: `admin` (runs the software: what it
   does, how to set it up, what to check when it goes wrong), `developer` (changes or extends
   the code), or `both` (default: admin-level first, then "Under the hood"). Present:
   ```
   Topic:        topics/<slug>.md, audience <…>: one line on what it covers
   New pages:    flows/…, components/…, decisions/… (one line each, and why)
   Updated:      existing pages that get a section or a link (what changes)
   Glossary:     ~N new terms     Gotchas: ~N
   Questions:    anything only the user can answer (policy, history, intent)
   ```
   Wait for confirmation. The user may add insights here; treat them as section B.
5. **Write**, following the templates in `content-model.md`:
   - the **topic page** first: the entry point that ties the subject together;
   - new **flows** for journeys the subject adds (a purchase end to end, a callback, a refund);
   - a new **component** only if the subject is a distinct body of code no component covers
     (give it `paths`, and remove those paths from any component that claimed them);
   - gotchas, glossary terms and decisions you found along the way;
   - small additions to existing pages (a paragraph, a link), not rewrites. Skip `locked` pages
     and tell the user what they'd need.
   Mark anything inferred, especially admin-facing claims you worked out from code (menu
   locations, option names, what a symptom means).
6. **Link it in.** Add the topic to the overview's `## Topics` section (create the section after
   "Main journeys" if missing) and to the reading guide if it's a common goal. Topic pages list
   their `components`; component pages pick up the link automatically.
7. **Finish:** `T manifest`, `T check` (fix what it reports). Report the pages added and
   changed in a few lines, and the open questions that only the user can answer.

## B. An insight

The user's own knowledge is the most valuable thing in the docs, because nothing else can
recover it. Record it faithfully, in their words where possible, and never present it as
something you worked out.

1. **Find where it belongs.** Search the docs for the subject it's about. It goes where a reader
   would need it, often in more than one place (a gotcha *and* the flow step it affects).
2. **Record it** in the form that fits:
   - a trap or warning → a `## ` entry in `gotchas.md`, plus a callout in the affected page;
   - why something is the way it is → a decision (`status: stated`), or an update to an inferred
     one (change its status to `stated` and its text to match);
   - a fact about a page's subject → an insight callout in that page:
     ```
     > [!INSIGHT]
     > Payment callbacks fail behind Cloudflare unless the provider's IP ranges are allowed.
     ```
   - a term → the glossary.
3. **Reconcile.** If the insight contradicts something the docs say (especially anything marked
   inferred), correct that text and say so. If it contradicts the code, don't silently pick one:
   record the insight, note the mismatch in a `> [!WARNING]`, and tell the user.
4. **Add code refs** only if you've confirmed them (`T check` will verify).
5. **Finish:** `T manifest`, `T check`. Tell the user exactly where it went (page and section).

## Keeping additions safe

`> [!INSIGHT]` blocks, `status: stated` decisions and `locked` pages are the user's. Every
command that rewrites pages (`init` refresh/regenerate, `update`) must carry insight blocks and
stated content over unchanged. Topic pages are refreshed like any other page, with the same rule.
