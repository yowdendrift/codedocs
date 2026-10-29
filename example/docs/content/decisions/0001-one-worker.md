---
title: One long-lived encode worker
status: inferred
components: [render-pipeline]
generated_with: 0.1.0
updated: 2026-10-28
---
# One long-lived encode worker

## Context
Encoding is slow and would freeze the editor if it ran on the UI thread.

## Decision
A single worker is created when the module loads and reused for every job.

## Alternatives
A worker per job would allow parallel exports and simpler message handling, at the cost of
startup time and memory (inferred).

## Consequences
Jobs must run one at a time, and the queue can swap the worker's message handler per job.

## Evidence
> [!INFERRED]
> The worker is created at module level in `src/render/queue.ts` and never terminated, and
> `src/render/queue.ts#RenderQueue.pump` refuses to start a job while one is running.
