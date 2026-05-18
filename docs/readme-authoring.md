# README authoring rules

This guide defines how we structure the repository `README.md` so it stays useful as a GitHub landing page instead of turning into a full manual.

## Purpose

The README should help a first-time visitor answer these questions quickly:

- What is this project?
- Why would I use it?
- How do I get it running?
- How do I connect to it?
- Where do I go for deeper docs or help?

Everything else should be compressed, summarized, collapsed, or moved into deeper docs.

## Core rule for `<details>`

Use `<details>` for optional depth, not for critical onboarding.

That means:

- keep the first-pass path visible
- collapse advanced or secondary material
- let the reader choose when to expand for more detail

GitHub supports `<details>` and recommends it for streamlining Markdown. This repo uses it to keep the README scrollable while still keeping important context nearby.

## What stays visible

These sections should stay expanded in the main README unless there is a strong reason not to:

- project name and one-sentence description
- quick start
- one working configuration example
- one working client connection example
- short explanation of what the project does
- short explanation of the security model
- support and maintainer information
- license

## What should usually go in `<details>`

Good candidates for collapsed sections:

- extra client examples
- build-from-source instructions
- long field-by-field configuration notes
- starter profile inventories
- architecture diagrams
- screenshots
- FAQ excerpts
- capability lists
- extended philosophy or design rationale
- previews of deeper docs

## What should move to deeper docs

If a section becomes long even after summarizing it, move the full explanation to `docs/` and keep only:

- a short summary in the README
- a collapsed preview when helpful
- a direct link to the full document

The README should stay a landing page. Deeper docs should carry the full reference load.

## `<details>` authoring rules

When using `<details>`:

1. Always include a clear `<summary>` label.
2. Make the summary text specific, such as `LibreChat example` or `Rule validation model`.
3. Keep one topic per collapsed section.
4. Put the most useful sentence before the collapsed section whenever possible.
5. Do not hide warnings or must-follow setup steps inside `<details>`.
6. Use `<details open>` only when there is a strong reason to default the section open.

## Recommended README shape

Use this structure as the default:

1. Single H1
2. One-sentence value proposition
3. Quick links
4. Quick start
5. Short project explanation
6. Short feature and boundaries summary
7. Configuration
8. Client connection
9. Security model
10. Docs with collapsed previews
11. Support
12. License

## Editing test

Before merging README changes, check:

- Is there still only one H1?
- Can a new visitor deploy from the visible sections without opening any collapsed content?
- Are collapsed sections truly optional?
- Did we summarize before linking away?
- Did the README get shorter or easier to scan, not just more segmented?
