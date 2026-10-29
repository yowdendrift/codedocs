# /codedocs explain [range | branch]

Explain a change to a person who has to review or own it, in terms of the docs they already know:
"this changes step 3 of the export flow", not "this edits queue.ts". Read `writing.md` if you
haven't this session.

`T` means `"<skill>/bin/codedocs"` (`<skill>` is defined in SKILL.md), run from the repo root.

## 1. Pin down the change

- A branch name: the range is `<base>...<branch>`, where base is the branch it came from
  (usually main; ask if unclear).
- A range (`HEAD~3`, `abc123..def456`): use it.
- Nothing given: uncommitted changes if there are any, otherwise the last commit. Say which you
  picked.

Run `T affected <range>` (or plain `T affected` for uncommitted work) to get the changed files,
their components, and the doc pages that describe them. Then read the diff, and the affected
pages' relevant sections, so you can describe the change against them. Read surrounding code only
where the diff alone is ambiguous.

If the repo has no docs yet, still explain the change, but say that `/codedocs init` would let you
anchor it to components and flows.

## 2. Write the explanation

Save it as `content/changes/YYYY-MM-DD-<slug>.md` with frontmatter `title`, `summary`,
`components: [...]`, `range`, `generated_with`, `updated`, so it shows in the viewer under
Changes. Use this structure, dropping empty sections:

~~~
# <What the change does, in plain words>

## In short
One paragraph: what changed, why (if known), and the effect a user or developer will notice.

## Where it lands
Which components and flows it touches, each linked, with one line on what's different there.
If it changes a flow, say which step(s).

## Choices worth checking
Decisions embedded in the change that a reviewer might not expect or might disagree with: a new
dependency, a behaviour assumption ("empty input now returns an empty list"), a pattern that
differs from the concern pages, something done in a new place. Each with a code ref and what it
affects. This is the most important section: it's where surprises hide.

## Reading order
| priority | where | why |
|---|---|---|
| read carefully | `src/…#Symbol` | core logic / risk |
| skim | `src/…` | wiring |
| ignore | `…` | generated, formatting, renames |

## The tricky part
At most two short deep dives on the hardest pieces: what it does, why this way, what would
break it.

## Check your understanding
Three questions the reader should be able to answer if they've understood the change, e.g.
"What happens now if an export is cancelled halfway?" No answers given; if they can't answer
one, that's the section to reread.

## How to try it
Steps to see the change working by hand.

## Effect on the docs
Which pages are now out of date, or "none".
~~~

## 3. Finish

1. `T manifest` so the change appears in the viewer.
2. In chat: two or three lines (what it does, the top "choice worth checking", where the file is).
3. If "Effect on the docs" lists pages, offer `/codedocs update`. Don't update them unasked.
