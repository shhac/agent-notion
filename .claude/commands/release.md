---
description: Release via tag push — CI builds, publishes, and bumps the Homebrew formula
argument-hint: <patch|minor|major>
---

# Release

Releasing `agent-notion` is automated. Pushing a `v*` tag triggers
`.github/workflows/release.yml`, which calls the shared `go-release` workflow in
`shhac/homebrew-tap` to cross-build every platform, publish the GitHub Release,
and regenerate + push `Formula/agent-notion.rb` (with shell completions) to the tap.
The tag also triggers `.github/workflows/publish-skill.yml`, which publishes
`skills/agent-notion` to `shhac/agent-skills`. **No manual build, and no manual
formula bump.**

## Steps

1. `$ARGUMENTS` must be `patch`, `minor`, or `major` — else stop and ask.
2. Pre-flight (CI re-runs tests on the tag, but check locally first):
   - Clean tree (`git status --short`), on `main`, up to date with `origin/main`.
   - Tests, vet, and lint pass (`make test`, `make vet`, `make lint`). The
     version is injected from the tag (`-ldflags -X main.version=…`) — there is
     no version file to edit.
3. Compute the new version by bumping the latest tag
   (`git describe --tags --abbrev=0`): patch → x.y.(z+1), minor → x.(y+1).0,
   major → (x+1).0.0.
4. Tag and push — this is the whole release:
   ```bash
   git tag "v${new_version}"
   git push origin "v${new_version}"
   ```
5. Verify CI and the outputs:
   ```bash
   gh run watch --repo shhac/agent-notion          # release + Publish skill runs green
   gh release view "v${new_version}" --repo shhac/agent-notion   # 6 assets
   ```
   Install / upgrade: `brew install shhac/tap/agent-notion` · `brew upgrade shhac/tap/agent-notion`

## Manual fallback (only if the workflow itself is broken)

Re-run a failed release with `gh run rerun <id> --repo shhac/agent-notion`. To bypass
the workflow entirely, build the `GOOS/GOARCH` binaries with
`-ldflags "-s -w -X main.version=<v>"`, `gh release create` the tarballs, and edit
`Formula/agent-notion.rb` by hand (see this file's git history for the old full flow).

## Secrets

The formula push authenticates via the `TAP_DEPLOY_KEY` secret in this repo's
`homebrew-tap` GitHub environment, paired with the read-write
"agent-notion release automation (env-scoped)" deploy key on `shhac/homebrew-tap`;
the skill publish uses the repo-level `SKILLS_DEPLOY_KEY`. If the workflow logs
"TAP_DEPLOY_KEY not set — skipping tap update", rotate the pair — pipe the
private key, never echo it:

```bash
ssh-keygen -t ed25519 -N "" -C "agent-notion release automation" -f tap_key
gh repo deploy-key add tap_key.pub -R shhac/homebrew-tap --allow-write \
  --title "agent-notion release automation (env-scoped)"
gh secret set TAP_DEPLOY_KEY --repo shhac/agent-notion --env homebrew-tap < tap_key
rm tap_key tap_key.pub
```
