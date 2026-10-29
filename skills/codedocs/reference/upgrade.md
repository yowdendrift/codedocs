# /codedocs upgrade

Move a repo's docs to the installed skill version. Content and look are kept.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

1. `T status`: note the docs' `skill_version` and the installed version.
   - Same version: say so. Offer `T install-viewer` anyway if the user has been editing the skill
     without bumping the version.
   - Docs newer than the skill: stop and tell the user (they're running an older install).
2. Read `<skill>/CHANGELOG.md` for the versions in between. Note anything marked
   **content model** (a change to folders, frontmatter, ref or link formats) or **viewer**.
3. `T install-viewer`: replaces `index.html` and `viewer.css` with the copies built into the
   binary (`serve` always uses its built-in copy anyway; the files matter when the folder is
   hosted statically), keeps `theme.css`, updates
   `skill_version` in `docs.config.json`.
   - If the changelog says the default theme gained new variables, add them to the repo's
     `theme.css` with the values set in the built-in default theme (`viewer.css` has fallbacks,
     so this is optional), without changing the user's existing values. Only add variables the
     changelog lists as theme variables; never add ones it mentions as optional caps or limits
     (like `--page-max-width`), and never copy values out of comments.
4. If there were **content model** changes, apply them mechanically to existing pages (rename a
   frontmatter key, move a folder) and say what you changed. Don't rewrite page content here.
5. `T manifest`, `T check`. Fix what's reported.
6. Report the version change, what changed in the viewer, and which pages are now "outdated"
   (written with older page guidance). Suggest `/codedocs init` → "Refresh outdated pages" when the
   changelog says the writing guidance or templates improved enough to be worth it.
