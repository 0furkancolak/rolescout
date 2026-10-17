# Release Checklist

1. **Scope**: every change since the last release is in `CHANGELOG.md` under
   `[Unreleased]`, with breaking changes and required operator actions called
   out.
2. **Checks on `main`**: CI is green (api, web, e2e). Locally, if needed:
   ```bash
   cd api && gofmt -l . && go vet ./... && go test -race ./...
   cd web && npm ci && npm run lint && npm test && npm run build && npm audit
   ```
3. **Clean installation**: from a fresh clone, follow
   [installation](installation.md) word for word on a machine or in a
   directory without previous data, then run `scripts/smoke.sh`. Fix the
   documentation if any step differs.
4. **Upgrade path**: start the previous release with data, back it up,
   upgrade as in [operations](operations.md#upgrades), run the smoke test,
   and confirm a restore of the pre-upgrade backup works.
5. **Security**: `govulncheck` and `npm audit` are clean or their findings are
   documented in [security](security.md); base images rebuilt with `--pull`.
6. **Version**: move `[Unreleased]` entries to a new version heading with the
   date, update the compare links, commit, then tag:
   ```bash
   git tag -a v0.2.0 -m "RoleScout 0.2.0"
   git push origin main v0.2.0
   ```
7. **Publish**: create the GitHub release from the tag with the changelog
   section as notes.
