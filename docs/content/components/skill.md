---
title: The skill
summary: The instructions the agent follows: SKILL.md dispatches each /codedocs command to a guide, and shared references define the content model and the writing style.
order: 80
paths: [skills/codedocs/SKILL.md, skills/codedocs/reference/**, skills/codedocs/agents/**, skills/codedocs/CHANGELOG.md, skills/codedocs/VERSION, skills/codedocs/REPO]
generated_with: 0.1.0
updated: 2026-10-29
---
# The skill

## Responsibility

The half of codedocs that runs in the agent: deciding what a codebase's components are, reading
code, and writing pages a human can learn from. It contains no code. It tells the agent which
[commands](commands.md) to run for the mechanical parts and how to write the rest. The
[content model](#the-shared-references) it defines is what the tool and viewer rely on.

## How it works

`skills/codedocs/SKILL.md` is loaded when someone runs `/codedocs …`. It sets the purpose (docs
for a human who didn't write the code), defines `<skill>` as the skill's own folder, maps the
first argument to one guide in `reference/`, and lists the rules every command follows: spend
tokens on understanding not bookkeeping, mark what's inferred, stamp pages with the guidance
version, never rewrite locked pages or the user's insights, finish with `manifest` and `check`.
The agent then reads only the guide for that command, so a `check` doesn't load the `init`
instructions.

The frontmatter sets `disable-model-invocation: true` (the skill only runs when asked, never on
its own; an `init` is expensive) and `allowed-tools` so the codedocs binary can run without a
permission prompt each time. `skills/codedocs/agents/openai.yaml` does the same job for Codex.

## Parts

### Command guides

One file per command in `skills/codedocs/reference/`, each starting by defining `T` as the tool
command and pointing back to the shared references:

- `skills/codedocs/reference/init.md`: inventory, an outline the user approves, then pages in a
  set order.
- `skills/codedocs/reference/update.md`, `skills/codedocs/reference/check.md` and
  `skills/codedocs/reference/explain.md`: keeping docs current and explaining changes.
- `skills/codedocs/reference/add.md`: a subject to research, or an insight to record.
- `skills/codedocs/reference/style.md`, `skills/codedocs/reference/upgrade.md` and
  `skills/codedocs/reference/links.md`: the viewer's look, version moves, and code-panel links.

### The shared references

`skills/codedocs/reference/content-model.md` is the contract: folders, frontmatter keys per page
kind, code ref and link formats, callouts, diagram rules and a template per page kind.
`skills/codedocs/reference/writing.md` is the style guide: purpose before mechanism, why over
what, observed versus inferred versus stated, length budgets, how to choose components and
flows. These two files shape every page more than anything else in the repo.

### Versions and changelog

`skills/codedocs/VERSION` is the product version (baked into the binary at build time).
`skills/codedocs/reference/VERSION` is the separate page-guidance version that pages record in
`generated_with`. `skills/codedocs/CHANGELOG.md` tags each change as viewer, content model,
guidance or theme, which tells `/codedocs upgrade` what to do. See
[versioning](../concerns/versioning.md).

## Interfaces

Drives the agent; calls the binary through the launcher in [distribution](distribution.md). The
content model is shared with the [docs model](docs-model.md) (which parses it) and the
[viewer](viewer.md) (which renders it).

## Watch out for

- Claude Code substitutes `${CLAUDE_SKILL_DIR}` only in SKILL.md, not in files the agent reads
  afterwards, which is why the guides say `<skill>` and SKILL.md defines it once.
- Changing a template or the writing guide is a guidance change: bump
  `skills/codedocs/reference/VERSION`, or existing pages won't show as outdated.
