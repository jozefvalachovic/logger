---
name: release
description: Prepare a new logger version for release — bump version.go, document the changes in README.md, and run the full CI verify chain. Stops before pushing.
disable-model-invocation: true
argument-hint: "[patch|minor|major or X.Y.Z]"
---

# Release prep

Prepare the next version of `github.com/jozefvalachovic/logger/v4`. Pushing to `main` makes CI
create the `v<Version>` tag from `version.go`, so this skill ends with a local commit, never a push.

## Steps

1. **Find what changed.** Read `version.go` for the current version, then
   `git log --oneline v<current>..HEAD` and `git diff v<current>..HEAD --stat` to see what is unreleased.
   If nothing user-visible changed, say so and stop.

2. **Pick the new version.** Use `$ARGUMENTS` if given (`patch`, `minor`, `major` or an explicit
   `X.Y.Z`). Otherwise propose one: new exported API → minor; fixes only → patch. A major bump means
   a new module path (`/v5`), so never pick it on your own — ask.
   Confirm the tag doesn't exist yet: `git ls-remote --tags origin "refs/tags/vX.Y.Z"`.

3. **Bump** `const Version` in `version.go`.

4. **Update README.md.**
   - Add a `### vX.Y.Z Changes` section directly above the previous one (under "Migration from v3"),
     with one bullet per user-visible change, written in the style of the existing entries.
   - If exported API was added or changed, update the **API Reference** section and any
     feature section that documents it.
   - If files were added or removed, update the **Package Structure** tree.

5. **Verify** — every step must pass:
   ```sh
   gofmt -l . && go vet ./... && go fix -diff ./... && golangci-lint run && go test -race -count=1 ./...
   ```

6. **Commit** on `main` (no branch, no PR, no co-author trailer):
   `chore: release vX.Y.Z`

7. **Stop.** Report the version, the README entries, and the verify results. Tell the user that
   `git push origin main` will trigger the tag and the GitHub release. Do not push.
