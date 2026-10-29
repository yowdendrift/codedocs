# /codedocs check

Find and fix drift between the docs and the code. The detection is free; you only spend tokens
on what it flags.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

1. Run `T manifest`, then `T check --json`.
2. If there are no issues, no coverage gaps worth mentioning, and no outdated pages: say the docs
   are in sync (and how far behind HEAD they are, from `T status`) and stop.
3. For each issue, fix the page line it points to:
   - `missing-file`: the file moved or was deleted. Find where it went (`git log --follow
     --name-status -- <old path>` or a quick search for the main symbol) and update the ref. If
     the code is truly gone, update the text that relied on it.
   - `missing-symbol`: renamed or removed. Search for the likely new name; update the ref and any
     prose that changed meaning.
   - `line-out-of-range`: re-locate the spot and, if there's a symbol for it, switch to a
     `#symbol` ref.
   - `broken-link`: fix the link to the page's current path.
   - `unknown-component`: a `components:` entry that matches no component slug. Fix the slug.
   - `component-without-paths`: add `paths` globs.
   Don't re-read whole files: look only at the spots involved.
4. **Coverage:** list uncovered directories for the user, with your suggestion for each: extend a
   component's `paths`, add an `ignore` glob (generated or vendored code), or document a new area
   (suggest `/codedocs update` or a new component page; don't write it unasked).
5. **Outdated pages** (written with older page guidance): mention them and suggest
   `/codedocs init`, mode "Refresh outdated pages".
6. Re-run `T manifest` and `T check` after fixing. Report what was fixed in a few lines.
