---
description: Build, release, and publish to Homebrew
argument-hint: <patch|minor|major>
---

# Release

Release the `agent-notion` CLI. A `v*` tag push triggers the shared GitHub
workflow (`shhac/homebrew-tap/.github/workflows/go-release.yml`), which
cross-builds, publishes the GitHub Release, and regenerates + pushes the
Homebrew formula. There is no version file — the binary version comes from the
git tag via ldflags.

## Arguments

- `$ARGUMENTS` — version bump type: `patch`, `minor`, or `major`

## Instructions

### Pre-flight

1. Confirm the working tree is clean (`git st`) and you are on `main`, up to
   date with `origin/main`. If not, stop and ask.
2. Run `make test`, `make vet`, and `make lint`. If any fails, stop and fix.
3. Compute the new version: latest tag from `git tag --sort=-v:refname | head -1`,
   bumped per `$ARGUMENTS`. Show the user current → new before continuing.

### Step 1: Tag and push

```bash
git tag v<NEW_VERSION>
git push origin main v<NEW_VERSION>
```

### Step 2: Watch the release workflow

```bash
gh run watch --exit-status $(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```

If the workflow fails, inspect with `gh run view --log-failed` and fix before
re-tagging (delete the tag locally and remotely first).

The tap publish requires the `TAP_DEPLOY_KEY` secret in this repo's
`homebrew-tap` environment; if the workflow logs say the secret is unset, the
formula update was skipped intentionally.

### Step 3: Verify

```bash
gh release view v<NEW_VERSION>
```
