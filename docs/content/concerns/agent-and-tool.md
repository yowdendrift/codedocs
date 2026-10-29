---
title: Agent and tool
summary: The split that runs through everything: the agent spends tokens on understanding and writing; the binary does anything that can be computed.
order: 10
components: [skill, commands, code-intelligence, docs-model]
generated_with: 0.1.0
updated: 2026-10-29
---
# Agent and tool

## The pattern

Every command pairs an agent following a guide with a deterministic tool. The agent's job is
judgement: what the components are, why the code is shaped this way, what a newcomer needs to
hear. The tool's job is everything else: listing and counting files (`tool/cmds.go#cmdStats`),
mapping a change to pages (`tool/cmds.go#cmdAffected`), verifying refs and links
(`tool/cmds.go#cmdCheck`), finding where a subject lives (`tool/find.go#cmdFind`), building the
index the viewer reads (`tool/pages.go#buildManifest`). The guides tell the agent which command
answers each question, and rule 1 in `skills/codedocs/SKILL.md` forbids doing those things by
reading files.

> [!INSIGHT]
> The first version of this idea was a full "software factory" prompt, and it was dropped for
> being too token-inefficient. Keeping the agent to understanding and writing, and giving every
> mechanical step to a tool, is the answer to that.

## The rule

- If a question has one right answer that a program can compute, it's a tool command, not
  instructions for the agent.
- Tool output is short plain text an agent can read cheaply, with `--json` when something needs
  parsing.
- The tool never writes prose. It reports; the agent decides and writes.
- Each guide ends with `manifest` and `check`, so the agent's work is verified by the tool before
  it reports back.

## Where it applies

- [The skill](../components/skill.md): every guide names the tool command for each mechanical step.
- [Bookkeeping commands](../components/commands.md) and
  [code intelligence](../components/code-intelligence.md): the tool side.
- [Docs model](../components/docs-model.md): the format both sides share.

## Exceptions

Reading code to understand it stays with the agent, including following imports in a small repo.
`find` narrows what to read; it doesn't summarise.
