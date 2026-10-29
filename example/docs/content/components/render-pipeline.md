---
title: Render pipeline
summary: Turns a timeline snapshot into a video file on a background worker, one job at a time.
order: 20
paths: [src/render/**]
generated_with: 0.1.0
updated: 2026-10-28
---
# Render pipeline

## Responsibility
Queueing, running and cancelling exports. It never touches the live timeline: it only sees the
snapshot it was given.

## How it works
`src/render/queue.ts#RenderQueue` keeps a list of jobs. When nothing is running it posts the next
queued job to a single worker, which reports progress and finally a result or an error.

```mermaid
stateDiagram-v2
  [*] --> queued
  queued --> rendering
  queued --> cancelled
  rendering --> done
  rendering --> failed
```

## Parts
### Render queue
Runs on the UI thread. `enqueue` adds a job and starts it if idle; `pump` picks the next one;
`finish` frees the slot (`src/render/queue.ts#RenderQueue.pump`).

### Encode worker
Runs off the UI thread and does the slow work frame by frame (`src/render/encode.worker.ts`).
It checks for cancellation between frames.

## Interfaces
Messages to the worker: `render` and `cancel`. Messages back: `progress`, `done`, `error`.

## Watch out for
Cancelling a running job comes back as a **failed** job with the message "cancelled", not as
`cancelled` (inferred: probably an oversight, as the queue has a `cancelled` state it never uses
for running jobs).
