# Writing guide

The reader is a capable developer who has never seen this codebase and wants a working mental
model fast. Write for them.

## What to write

- **Purpose before mechanism.** Say what a thing is *for* before how it works.
- **Shape before detail.** The overview and component pages give the shape; flows give the
  detail. Don't drag implementation detail up into the overview.
- **Why over what.** The code already says what it does. The docs add why it's like this, how the
  parts relate, what must stay true, and where the traps are.
- **Name things consistently.** Use the glossary's term every time. If the code uses two names
  for one thing, say so once in the glossary and pick one for the docs.
- **Concrete over abstract.** "A clip is a slice of a media file placed on a track at a start time"
  beats "a clip encapsulates temporal media placement".
- **Point, don't paste.** Use code refs (`src/x.ts#Thing`) instead of pasting code. Paste a few
  lines only when the snippet *is* the explanation (a data shape, a tricky condition).

## How to write it

- Plain sentences, active voice, present tense. Short paragraphs.
- No filler: no "This document describes…", no "It is important to note that…", no marketing.
- Lists for parallel items; prose for explanations and reasoning.
- Every page must make sense on its own to someone arriving from search: first sentence of each
  section carries the point.
- Be honest about mess. If something is tangled, duplicated or half-finished, say so plainly and
  add it to gotchas. A newcomer needs to know where the swamp is.

## Inferred vs stated

You'll often be documenting code nobody explained to you. Separate:

- **Observed**: what the code does. State it plainly; it's checkable.
- **Inferred**: why it's like that, what was intended, whether something is deliberate. Mark it:
  `(inferred)` in a sentence, a `> [!INFERRED]` callout for a paragraph, `status: inferred` on a
  decision. Give the evidence ("the worker is created once at startup and never torn down, which
  suggests…").
- **Stated**: what the user told you. In a decision, `status: stated`. Elsewhere, put it in an
  `> [!INSIGHT]` callout so it shows as theirs and survives every rewrite.

Never invent a reason to make a page sound complete. "The reason isn't clear from the code" is a
useful sentence.

## Writing for admins

Topic pages for `admin` or `both` start with sections for people who run the software rather
than change it.

- Describe what it does in terms they see: screens, options, emails, log entries. No code refs
  in admin sections; put those in "Under the hood".
- Name the place in the UI the way the UI does ("Setup › Payment profiles"). If you worked it
  out from code (routes, templates, phrases) rather than seeing it, mark it `(inferred)`.
- "What can go wrong" is the most useful section: write it as symptom → likely cause → where to
  look. Real failure modes found in the code (unhandled states, retries, timeouts) belong here.
- Keep the admin part readable on its own; someone who never scrolls to "Under the hood" should
  still get a correct picture.

## Length budgets

Long docs don't get read. Aim for:

| page | target |
|---|---|
| overview | 1–2 screens |
| component | 1–3 screens; split a component if it needs more |
| flow | "In short" ≤ 5 sentences; each step ≤ 6 lines |
| concern | ≤ 1 screen |
| decision | ≤ 1 screen |
| topic | 1–3 screens; link out rather than repeat flows and components |
| glossary entry | 1 sentence |

## Choosing what to document

- **Components:** the 4–10 things a developer would draw as boxes on a whiteboard. Fewer, larger
  components beat many tiny ones; their sub-parts go in "Parts".
- **Flows:** 3–6 journeys, chosen by how often they run and how risky or complex they are. Always
  include the main thing a user does, and whatever has the most moving parts.
- **Concerns:** only patterns that genuinely cut across several components (state management,
  errors, threading/async, persistence, undo, config, logging). Skip ones that don't exist.
- **Decisions:** only choices that shape the code and that someone might reasonably question or
  undo by accident.
- **Gotchas:** anything that surprised you while reading. If it surprised you, it will surprise them.
