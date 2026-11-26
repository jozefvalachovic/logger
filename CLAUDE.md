# logger

Zero-dependency Go structured logging library (module `github.com/jozefvalachovic/logger/v4`).
Keep go.mod free of `require` lines — gRPC/OTel/SQL support is built on stdlib interfaces only.

## Verify (mirror CI; all must pass before a change is done)

```sh
gofmt -l . && go vet ./... && go fix -diff ./... && golangci-lint run && go test -race ./...
```

CI pins Go 1.26 and golangci-lint v2.8.0 (config in `.golangci.yml`). Local tools are newer,
so don't use APIs or language features newer than Go 1.26.

## Architecture

- Pipeline: `Log*` → `logInternal` (level, sampling, dedup, metrics, async) → `redactHandler` →
  `slog.NewMultiHandler(prettyHandler + Config.AdditionalHandlers)`. Redaction must stay outermost
  so no destination ever sees unredacted values.
- Global state lives in `atomic.Pointer`s (`globalConfig`, `defaultLogger`, `metrics`, `auditLogger`);
  writers serialise on `configWriteMu`. The log path must never take that lock.
- Zero-value `Config` fields mean "use default" — that's why `LevelSet` / `SampleRateSet` exist.
  New fields whose zero value is meaningful need the same treatment.
- `audit/` must not import the root package (the root package imports `audit`).

## Tests

Tests mutate the global logger via `SetConfig` — no `t.Parallel()`, and restore config when done.
Concurrency changes need a test that exercises them under `-race`.

## Public API and releases

- v4 is backward compatible with v3: never remove or change exported signatures.
- Every push to `main` auto-tags `v<Version>` from `version.go` (skipped if the tag already exists).
  Any change to shipped code must bump `version.go` and add a `### vX.Y.Z Changes` section to
  README.md (plus the API Reference / Package Structure blocks if affected). Use `/release`.

## Git workflow

- Commit directly to `main`. Never create branches or pull requests.
- No `Co-Authored-By` or other AI attribution trailers in commit messages.
- Never push or tag unless explicitly asked — a push to `main` can cut a release.
