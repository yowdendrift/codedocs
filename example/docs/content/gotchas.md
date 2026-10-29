---
title: Gotchas
summary: Traps that will catch you out.
generated_with: 0.1.0
updated: 2026-10-28
---
# Gotchas

## Tracks can be edited without notifying anyone
`Timeline.tracks` is public, so code can push to it directly, as `src/main.ts` does. Listeners
aren't told, and the UI won't redraw. Add a method for adding tracks before relying on this.

## A cancelled render reports as failed
See [render pipeline](components/render-pipeline.md#watch-out-for).
