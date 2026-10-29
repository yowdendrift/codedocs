---
title: Glossary
summary: The project's nouns.
generated_with: 0.1.0
updated: 2026-10-28
---
# Glossary

| Term | Meaning | Where |
|---|---|---|
| Clip | A slice of a media file placed on a track at a start time. | `src/timeline/timeline.ts#Clip` |
| In point | Where in the source media a clip starts. | `src/timeline/timeline.ts#Clip` |
| Job | One export request, with its state and progress. | `src/render/queue.ts#RenderJob` |
| Snapshot | A deep copy of the tracks, taken so rendering can't see later edits. | `src/timeline/timeline.ts#Timeline.snapshot` |
| Track | An ordered lane of clips, video or audio. | `src/timeline/timeline.ts#Track` |
