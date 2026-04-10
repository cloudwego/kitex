Read the actual code diffs. Do not trust commit messages or existing changelogs
— they can be vague, misleading, or wrong. Determine real impact from the code.

Rules:

1. Sections in order (omit empty):
   - ## Breaking Changes
   - ## Deprecations
   - ## Features
   - ## Fixes
   - ## Improvements
   - ## Deps
   - ## Commits

2. Entry format: `- pkg/area: description (#PR or short hash)`
   - The `pkg/area` prefix MUST be an actual Go package name in this repo
     (e.g., `client`, `server`, `remote`, `grpc`, `codec/thrift`, `retry`).
     Do NOT use: informal names (`meshheader`, `metahandler`, `resolver`,
     `timer`, `wpool`), or wrong casing (`gRPC` — use `grpc`).
     If a PR touches multiple packages, an abstract label is acceptable
     (e.g., `perf`, `deps`), but prefer a specific package name when possible.
   - Descriptions must be short and concise.
   - Use fully qualified names for symbols (`pkg.Type.Method`).
   - Collapse related commits into one entry.

3. Per-section rules:
   - **Breaking Changes** — list changed symbols with before/after signatures:
     `- pkg: Change \`Func(old)\` -> \`Func(new)\` (#123)`
     Indent a migration note only when the migration is non-obvious.
     Omit if the change is self-explanatory (e.g., add/remove an argument, remove usage).
   - **Deprecations** — note the replacement with full symbol path.
   - **Features** — include new func/type names
     (e.g., "Add `http.Client.Send` for async messaging").
   - **Fixes** — include all bug fixes regardless of size.
   - **Improvements** — major behavior/performance changes only.
   - **Deps** — `Bump pkg v1.2 -> v1.4`, `Add pkg v1.0`, or `Remove pkg`.

4. Skip:
   - CI/CD, docs, typos, formatting, refactors with no behavior change.
   - Internal/private changes not part of the public API.
     Exception: if an internal type appears in a public API signature (returned
     by or passed to a public func), include it — note `(exposed via pkg.Func)`.

Example output:

```
## Breaking Changes
- http: Change `Server.Listen(addr string)` -> `Server.Listen(addr string, opts ...Option)` (#201)
  Migration: pass no opts for old behavior.

## Deprecations
- log: Deprecate `Print`, use `Logger.Info` instead (#199)

## Features
- http: Add `Client.Send` for async request dispatch (#198)

## Fixes
- auth: Fix token refresh race on concurrent requests (#205)

## Improvements
- cache: Reduce memory usage via LRU eviction (#210)

## Deps
- Bump golang.org/x/net v0.20 -> v0.25 (#212)
- Add github.com/redis/go-redis v9.0 (#214)
- Remove github.com/pkg/errors (#215)
```

5. The diff range for each version is defined in `changelog_task.md`.
   Use `git log --oneline <from>..<to>` to get all commits.

6. The last section `## Commits (<from>..<to>)` lists ALL commits (one per line)
   in the range, with the diff range in the heading:
   ```
   ## Commits (v0.1.0..v0.2.0)
   - abcd123 feat(client): add streaming support (#198)
   - ef01234 fix(retry): data race in container init (#205)
   - ...
   ```

Output only the changelog markdown. No preamble, no summary.
