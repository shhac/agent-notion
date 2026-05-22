# design-docs/

Long-form notes that don't belong in code or PR descriptions.

This folder is a **journal**, not a wiki. Most entries are snapshots in time — dated, version-pinned, written in past tense. They don't get rewritten when reality moves on; new entries get added instead. That keeps old entries useful as history rather than dangerous as out-of-date documentation.

## Structure

- **`decisions/`** — choices made, with the alternatives we considered and why we chose what we chose. One decision per file, prefixed with the date (`2026-02-v3-token-expiry-handling.md`). Immutable after merge: if we revisit, that's a new dated file that supersedes the old one. Each file states its own date in the header so it's clear when read in isolation.
- **`reference/`** — point-in-time captures of external surfaces (API endpoints, wire formats, behaviours we observed). Each carries a "captured on" date and the version of the thing being captured (`notion-client-version 23.13.20260519.1354`, etc.). When the external surface meaningfully changes, the response is a new dated capture, not an in-place edit.
- **`concepts/`** — the only place where editing in place is the right move. Short, living docs that explain how a piece of the system fits together at a high level (e.g. "why we have two backends"). Kept narrow so they don't drift. Each carries a "last reviewed" date.

## What goes in here

Yes, save:
- Decisions worth their own ADR (alternatives, rationale, what we chose)
- Learnings from investigation (something surprising about the world that took effort to find out)
- Reasoning behind non-obvious code structure
- Design goals
- High-level architectural concepts
- Captured API/protocol shapes from external systems we depend on

No, don't save:
- Real-world data: user IDs, page IDs, real timestamps, organisation names, email addresses, token fragments, integration inventories
- Script dumps or full HAR file recordings (extract the shapes/decisions — drop the raw)
- Documentation that duplicates the README, the skill, or the code itself
- Implementation logs with commit SHAs (that's `git log`'s job)
- Roadmaps for work that has already shipped

If a doc would lose value the moment its content rots, it doesn't belong here.

## Rules for new docs

1. **Date in the header.** "Captured 2026-05-22" or "Decision made 2026-02-17" — top of the file, not buried.
2. **Pin the version of what you're describing.** For external API snapshots: `notion-client-version`, package version, commit SHA, whatever identifies the snapshot's point in time. For code-internal docs: there's nothing to pin, but say so explicitly.
3. **Past tense or as-of tense, never evergreen present.** "We chose v3 because…" not "We use v3 because…". "Notion returned a 200 with empty results when the token had expired" not "Notion returns…". The grammar tells the reader they're looking at a snapshot.
4. **Reference code identifiers sparingly, and only as starting points.** Prefer "look for the archive op builder in `src/notion/v3/operations.ts`" over `archiveBlockOps at src/notion/v3/operations.ts:123`. Function names rename; line numbers shift. The doc shouldn't claim to know the current state of the codebase.
5. **No real-world data.** Synthetic IDs (`<pageId>`, `<userId>`), placeholder text, no identifiable workspace or user names. Test fixtures use the same rule (see `CLAUDE.local.md`).
6. **Append, don't edit.** Exception: `concepts/` docs are explicitly living and meant to be edited. Everything in `decisions/` and `reference/` is immutable after merge — if reality moves, write a new dated entry.

## When to write one

You don't need to write a doc for every change. The bar is "would a future reader (or future-you) lose useful context if this only existed as a commit message?"

A good fit:
- "We chose to do X instead of Y because Z" — when Y was the obvious choice and the reasoning isn't visible from the diff.
- "Here's what Notion's archive operation looks like over the wire" — when the upstream surface is undocumented and we did real work to find it out.
- "Here's why the auth flow is shaped the way it is" — when the architecture isn't readable from the code alone.

A bad fit:
- "Here's what `page archive` does." (Read the code or the help text.)
- "Here's what shipped this week." (Read `git log`.)
- "Here's a feature we might do later." (Open an issue.)
