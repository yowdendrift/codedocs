---
title: UI
summary: Buttons and wiring that connect the person using the editor to the timeline and render queue.
order: 30
paths: [src/ui/**, src/main.ts]
generated_with: 0.1.0
updated: 2026-10-28
---
# UI

## Responsibility
Startup wiring and user controls. It holds no state of its own.

## Parts
### Startup
`src/main.ts` creates one timeline with an empty video track and one render queue, then mounts
the controls.

### Export button
`src/ui/export-button.ts#mountExportButton` refuses to export an empty timeline, otherwise
enqueues a snapshot and shows the job number.
