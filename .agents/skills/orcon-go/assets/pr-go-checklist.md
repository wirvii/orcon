<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
## Go checklist

<!-- Paste this block into the description of every pull request that changes Go code, go.mod/go.sum, go.work, .golangci.yml, sqlc.yaml, SQL, the OpenAPI generator config or a Go CI workflow. -->

- [ ] The change derives from an accepted ADR or an approved spec (cite it: `SPEC-…`, `BL-…`, `docs/adr/…`).
- [ ] I loaded the `orcon-go` skill and followed its Workflow.
- [ ] `make check` exits 0 on this branch (same as `sh .agents/skills/orcon-go/scripts/verify.sh`).
- [ ] `make hooks` is enabled, so every commit message passed the provenance check.
- [ ] Code sits in the right package kind (area, adapter, platform, `cmd/orcon`); a new area or adapter adds its depguard rule in this change.
- [ ] Interfaces are small and declared by their consumer; errors are wrapped with `%w` and translated only at the edge.
- [ ] `ctx` is the first parameter of every blocking or I/O call; every goroutine has an owner that waits; concurrency is bounded.
- [ ] Logging uses the injected `*slog.Logger`; no secrets in logs or errors.
- [ ] Tests are table-driven with hand-written fakes; area coverage stays at 80 % or more.
- [ ] Storage changes: `make test-postgres` passes (both databases).
- [ ] SQL or API changes: sources edited, `make generate` run and the output committed.
- [ ] Hot-path changes: `make bench-compare` table pasted below; any regression over 10 % is justified.
- [ ] New dependencies: standard library considered first, license allowed (or an owner-approved line in `tools/licenses/exceptions.txt`), reason given below.
- [ ] Every `//nolint` names the linter and a reason; `.golangci.yml` was not weakened.
- [ ] Doc comments on exported identifiers; SPDX header with `The Orcon Authors` on every new file.
- [ ] Comments, commit messages and this description use neutral contributor wording and carry no AI authorship trailers.

### Dependencies added or upgraded

| Module | Version | License | Reason |
|---|---|---|---|
| | | | |

### Benchmarks (hot paths only)

<!-- Paste the benchstat table from make bench-compare. -->
