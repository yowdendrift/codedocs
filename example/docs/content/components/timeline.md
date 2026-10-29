---
title: Timeline
summary: The document being edited, tracks of clips placed in time, and the only source of truth for what gets exported.
order: 10
paths: [src/timeline/**]
generated_with: 0.1.0
updated: 2026-10-28
---
# Timeline

## Responsibility
Holds the edit: which media appears where and for how long. It does not know about rendering,
files or the UI.

## How it works
A `Timeline` holds a list of tracks; each track holds clips sorted by start time. Changes go
through methods that notify listeners, so the UI can redraw.

## Parts
### Clips and tracks
A clip is a slice of a media file placed on a track at a start time
(`src/timeline/timeline.ts#Clip`). Tracks are either video or audio (`src/timeline/timeline.ts#Track`).

### Change notifications
`src/timeline/timeline.ts#Timeline.onChange` registers a listener; every edit calls `emit`.

## Interfaces
- `src/timeline/timeline.ts#Timeline.snapshot` gives a deep copy for rendering.
- `src/timeline/timeline.ts#Timeline.duration` is the end of the last clip on any track.

## Watch out for
> [!WARNING]
> `tracks` is a public field. `src/main.ts` pushes a track into it directly, which skips change
> notifications. See [gotchas](../gotchas.md#tracks-can-be-edited-without-notifying-anyone).
