# Dual-backend architecture

> Concept doc — currently load-bearing. Last reviewed 2026-05-22.

The CLI talks to Notion through one of two backends, behind a common interface. Every command consumes normalized types; neither backend's wire format leaks out.

```
        CLI commands (page get, database query, …)
                       │
                       ▼
                NotionBackend interface
                       │
            ┌──────────┴──────────┐
            ▼                      ▼
    OfficialBackend          V3Backend
    (@notionhq/client)       (custom /api/v3/ client)
    OAuth or integration     token_v2 desktop cookie
    token                    (no programmatic refresh)
```

## Why two

The official REST API at `api.notion.com/v1` is the safe default — documented, supported, refresh-token aware. It covers most reads and writes adequately.

A handful of features the CLI cares about are not in the official API at all, or are far cheaper on v3:

- **Full-text content search.** The official `search` endpoint only matches titles. v3's `search` does real content search. This is the load-bearing reason v3 exists in this codebase — an LLM-facing CLI without content search is half-blind.
- **Single-request page tree.** `loadPageChunk` returns an entire page's blocks in one call. The official API requires recursive `blocks/{id}/children` traversal.
- **Backlinks, version history, activity log, real Archive, exports, AI chat.** Not in the official surface at all.

Backend-specific features are marked with `◆` in the CLI help. On the official backend, the corresponding commands throw with a pointer at `agent-notion auth import-desktop`.

## Trust model

| Backend | Auth | Refresh | Stability |
|---|---|---|---|
| Official | OAuth or internal-integration token | OAuth refresh-token rotation | Public contract; changes are versioned and announced |
| V3 | `token_v2` cookie from the desktop app session | None — re-import when expired | Internal API; shape can change between Notion releases |

The v3 backend pins a `notion-client-version` header to a recent desktop release. When Notion ships a backend change that breaks us, the symptom is usually a specific endpoint starting to 4xx — not silent corruption.

## When to add to which backend

When implementing a new feature:

1. If the official SDK can do it, do it there. Less risk, no desktop-session requirement.
2. If only v3 can do it, implement on `V3Backend` and throw `CliError` from `OfficialBackend` with a clear pointer at `auth import-desktop`. Mark `◆` in help.
3. If both can do it but v3 is materially cheaper (single-request page tree, etc.), implement both and let the runtime pick based on which backend is active.

The interface gets new methods either way — the normalization point is non-negotiable. CLI commands never know which backend they're talking to.

## Auth surface, in one paragraph

OAuth is the primary path: users register their own Notion integration, exchange via localhost callback, store `access_token` + `refresh_token` in macOS Keychain with a `__KEYCHAIN__` sentinel pattern in config. Internal-integration tokens are the no-OAuth fallback (single workspace, no refresh). Desktop `token_v2` is imported separately and lives alongside the official-backend credentials — switching backends per-command is a future option but not currently exercised. Multi-workspace lookup is a config map keyed by alias with a default pointer.

The detailed flows live in `src/cli/auth/` and `src/lib/{keychain,desktop-token,config}.ts` — those files are the source of truth, not this doc.
