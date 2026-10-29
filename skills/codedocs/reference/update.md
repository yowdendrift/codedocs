# /codedocs update [range]

Bring the docs in line with code that changed. Small, targeted edits only: this should cost a
fraction of `init`. Read `content-model.md` and `writing.md` if you haven't this session.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

1. **Find what changed.** Run `T affected` (or `T affected <range>` if the user gave one, e.g.
   `HEAD~3`, `main..feature`). With no range it covers everything since `last_synced`, plus
   uncommitted and untracked files. It lists changed files, the component each belongs to, the
   pages to review and why, and files no component claims. A page flagged because it
   *references* a changed file needs a proper look; one flagged only because it *involves* a
   changed component usually just needs its claims about that component sanity-checked.
   - Nothing changed: say so and stop.
   - No `last_synced` and no range: ask the user which range to use.
2. **Read only the change.** `git diff <base> -- <changed files>` for the affected files (plus
   the working tree if uncommitted). Open surrounding code only where the diff alone doesn't
   tell you what the change means.
3. **Update the pages listed, minimally.**
   - Fix statements the change made wrong; add what's new; remove what's gone.
   - Keep the page's structure and wording where it's still right. Don't rewrite for style.
   - A changed flow: update the affected steps and the diagram if the hops changed.
   - Skip `locked` pages; tell the user what they'd need to change.
   - Keep `> [!INSIGHT]` blocks and stated decisions as they are. If the change contradicts one,
     don't edit it: flag it to the user, who knows whether the insight still holds.
   - Bump `updated` and `generated_with` only on pages you actually changed.
4. **Things the change might add:**
   - A significant new choice (a new dependency, a new pattern, a reversed approach): write a
     decision record (`status: inferred` unless the user explained it).
   - A new trap: add it to gotchas.
   - New terms: add them to the glossary.
   - Unclaimed files: if they belong to an existing component, extend its `paths`. If they form
     a genuinely new area, propose a new component page to the user before writing it.
   - Overview: only touch it if the change alters the component map, a main journey, an
     invariant, or how to run the project.
5. **Finish.** `T manifest`, `T check` (fix what it reports), then `T mark-synced`. Report which
   pages changed, in a few lines.
