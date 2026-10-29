# /codedocs style <description>

Change how the docs look from a plain description: "dark, compact, monospace headings, green
accents", "like a printed book", "bigger text, more contrast".

1. Find the docs folder (`"<skill>/bin/codedocs" status`) and read its
   `theme.css`. It defines CSS variables for type, layout, colours (light and dark), page-kind
   colours and callouts.
2. Translate the description into changes to those variables first. Only add extra CSS rules,
   below the marker line at the end of the file, when variables can't express it (for example a
   different heading treatment or sidebar style). Read the docs folder's `viewer.css` to find the
   right selectors (it's a copy of the one built into the binary), but **never edit `viewer.css`
   or `index.html`**.
3. Keep both light and dark palettes working. If the user only describes one mode, derive the
   other to match. Keep body text contrast at 4.5:1 or better against its background, and check
   the code-ref, callout and kind colours stay readable on both.
4. Web fonts: allowed via an `@import` of Google Fonts at the top of `theme.css`. Always keep a
   system-font fallback in the stack.
5. If the user asks for a different look per project, edit that repo's `theme.css`. If they want
   it as their default for future repos, also write it to `codedocs/default-theme.css` in
   their config folder (`%APPDATA%\codedocs\` on Windows, `~/Library/Application Support/codedocs/`
   on macOS, `~/.config/codedocs/` on Linux; create the folder if needed). `install-viewer` uses
   that file instead of the built-in theme when it creates a new docs folder. Don't save it
   inside `<skill>`: updates replace that folder.
6. Tell the user to refresh the viewer (`/codedocs serve` if it isn't running). Summarise the change
   in a line or two.
