# 2026-02 — V3 token expiry: reactive 401 handling, not pre-flight validation

> Decision captured 2026-02-17. Outcome shipped shortly after.

## Context

Notion's `token_v2` desktop session cookie expires. When it does, most v3 endpoints respond with HTTP 401, which is easy to detect. **One endpoint diverges:** `POST /api/v3/search` was observed returning `200 OK` with `{"total": 0, "results": []}` for an expired token instead of 401 — silently failing as "no matches" rather than as an auth error.

Endpoints observed at the time:

| Endpoint | Expired-token response |
|---|---|
| `search` | `200 OK`, empty results |
| `loadUserContent` | `401 Unauthorized` |
| `getSpaces` | `401 Unauthorized` |
| `loadPageChunk` | `401 Unauthorized` |

## Options considered

**A. Pre-flight validation.** Before any v3 call, hit a known-401-on-expiry endpoint (`loadUserContent`) and cache the result for the session. Pros: catches the silent-search case everywhere. Cons: an extra round-trip on every CLI invocation.

**B. Heuristic empty-result detection.** Only validate when `search` returns `total: 0` for a non-empty query. Pros: zero overhead on the happy path. Cons: only fixes `search` — any other endpoint with the same silent-failure behaviour would still slip through; also adds latency on legitimately-empty searches.

**C. Token age check.** Track `extracted_at` on the stored session; warn or auto-refresh if older than ~30 days. Pros: no extra calls. Cons: revocation isn't age-bound; doesn't detect server-side invalidation.

## What shipped

Neither A, B, nor C. The chosen approach was **reactive 401-catch with a clear remediation message**: when any v3 call throws an unauthorized error, the CLI surfaces `"Desktop token expired. Run 'agent-notion auth import-desktop' to re-import."`

Rationale: the silent-`search` case is annoying but rare in practice (users notice "no results" and re-run with a known-good query, or hit a different command that 401s), and the cost of A — an extra round-trip on every invocation — felt disproportionate for a CLI where per-command latency is visible to the user.

## What this means going forward

- If silent-failure endpoints multiply (i.e. we hit other v3 endpoints that 200-with-empty-payload on expired tokens), revisit option A with caching.
- Option C is worth revisiting independently as a *proactive warning* signal (not a hard block) — separate from expiry detection.
- The reactive-only stance assumes 401 is the universal "expired token" signal. If Notion changes that contract, this decision needs re-examining.
