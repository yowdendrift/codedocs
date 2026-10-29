---
title: Error handling
summary: How failures are reported, which differs between the UI thread and the worker.
components: [render-pipeline, ui]
generated_with: 0.1.0
updated: 2026-10-28
---
# Error handling

## The pattern
Inside the worker, everything is caught and turned into an `error` message
(`src/render/encode.worker.ts`). On the UI thread, user mistakes are shown with `alert` and
programming errors are thrown (`src/timeline/timeline.ts#Timeline.addClip`).

## The rule
Worker code must never let an exception escape: the queue would wait for a reply forever.

## Exceptions
None yet.
