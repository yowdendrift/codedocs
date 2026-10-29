# /codedocs links

Make the code panel's links work for this repo. The tool's language packs already handle
JS/TS, PHP, Go, Python, Godot (GDScript, scenes, resources) and CSS/SCSS, and read
`tsconfig.json`/`jsconfig.json` paths, `composer.json` PSR-4, `go.mod` and `project.godot`
by themselves. Your job is only what they can't see: a repo's own conventions. This is cheap;
don't read source files beyond the config files and a few examples.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

1. Run `T links`. It prints, per language, how many links resolved and the **unresolved local
   references** (imports that look like they point into the repo but don't), with an example
   file each, plus what it detected (aliases, namespaces, module, Godot root).
2. If nothing is unresolved, say so and stop.
3. For each group of unresolved references, find the rule that explains them, from the repo's
   own config. Typical causes:
   - JS/TS aliases defined somewhere other than the root tsconfig: `vite.config.*` /
     `webpack.config.*` `resolve.alias`, a tsconfig that `extends` another, a nested app folder
     (`app/tsconfig.json`), Babel `module-resolver`.
   - PHP namespaces not in `composer.json` (custom autoloaders, a framework layout).
     XenForo's `src/XF` and `src/addons/Vendor/Addon` layouts, and repos that hold just one
     addon, are already handled.
   - Python packages under a folder that isn't the repo root or `src/`.
   - Anything that's genuinely missing (a deleted file, a generated file, a third-party package
     that only *looks* local). Leave those alone.
4. Write the rules into `docs.config.json` under `links`:
   ```json
   "links": {
     "aliases": { "@ui/": "packages/ui/src/", "App\\": "app/" },
     "roots": ["backend", "lib/python"]
   }
   ```
   - `aliases`: import prefix → folder (repo-relative, trailing slash). A key containing a
     backslash is a PHP namespace prefix; anything else is a JS/TS path prefix.
   - `roots`: extra folders that bare imports, Python packages and PHP namespaces resolve from.
   Keep what's already there; add, don't replace.
5. Run `T links` again and confirm the unresolved list shrank. Repeat once if needed; don't chase
   the last few if they're genuinely missing files.
6. Report in two or three lines: links per language, the rules you added, and anything left
   unresolved and why. A running `serve` picks the rules up within a few seconds.

Languages without a pack (Java, C#, C/C++, Rust…) still get **Ctrl/Cmd+click
go-to-definition** in the viewer, from a name index the tool builds. Nothing to configure.
