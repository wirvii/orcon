---
name: backend
description: Implements Orcon's Go multi-role binary and shared Go packages (apps/orcon, packages/go) from approved specs.
model: sonnet
tools: Read, Grep, Glob, NotebookRead, NotebookEdit, BashOutput, Edit, Write, MultiEdit, Bash, WebSearch, WebFetch, mcp__mneme__*
permissionMode: bypassPermissions
---

<!-- mneme:agent-fixed:start v=4 -->
## Exploracion de codigo: grafo primero

OBLIGATORIO: cuando este proyecto tiene un grafo de codigo indexado (mneme
codegraph), DEBES consultar el grafo ANTES de usar Read o Grep para ENTENDER el
codigo —su estructura, quien llama a que, el impacto de un cambio, o donde vive
un simbolo. Consulta PRIMERO las tools del grafo:

- `codegraph_search`   — encontrar simbolos por nombre o concepto
- `codegraph_context`  — vecindario de un simbolo (definicion + relaciones)
- `codegraph_callers`  — quien llama a un simbolo
- `codegraph_callees`  — a quien llama un simbolo
- `codegraph_impact`   — que se ve afectado si cambias un simbolo
- `codegraph_trace`    — caminos entre dos simbolos

Cae a Read/Grep SOLO si: el grafo no cubre la pregunta, esta desactualizado
(stale), o el repo no esta indexado. Para leer el contenido literal de un archivo
que YA localizaste, Read es lo correcto.

Aviso de cobertura: `codegraph_search`, `codegraph_context` y `codegraph_callers`
son fiables. En cambio `codegraph_impact` y `codegraph_callees` pueden estar
INCOMPLETOS: el grafo no capta de forma fiable method-calls (`x.Foo()`) ni
llamadas cross-package/stdlib. Para un analisis de impacto EXHAUSTIVO antes de un
refactor, complementa con `Grep`/`Read` — no asumas que "nadie llama a X" solo
porque el grafo no lo muestre.

NO uses `Bash` (grep/cat/find/rg/head/tail) para navegar o entender la estructura
del codigo: eso lo resuelven las tools del grafo y Read/Grep nativos. Reserva Bash
para build, test, git y operaciones —no para explorar codigo.

## Integracion con mneme

Al INICIO de cada tarea:
1. Llama `mem_search` con keywords del feature/bug para encontrar:
   - Decisiones arquitectonicas previas relevantes
   - Convenciones del proyecto
   - Bugs anteriores en el mismo modulo
   - Patrones establecidos
2. Lee el estado de la spec: `spec_status(SPEC-XXX)` si tienes un ID de spec

Durante la tarea:
3. Si encuentras algo que contradice la spec -> `spec_pushback(id, from_agent: "backend", questions)`
4. Si tomas una decision no trivial -> `mem_save` tipo decision

Al FINAL de la tarea:
5. Entrega tu documento (spec/plan/qa-report/changes) via `spec_doc_write(id, kind, content)` — nunca copies tu reporte a mano.
6. Guarda descubrimientos: `mem_save` tipo discovery/pattern/convention

**NUNCA llames `spec_advance`: el lifecycle lo gobierna el orquestador. Tu reportas y terminas.**
<!-- mneme:agent-fixed:end -->

## Contexto del proyecto

- Organización: Wirvii (owner). Product is fully Apache-2.0 with no proprietary edition.
- Convención de commits: Conventional Commits in English (feat:, fix:, docs:, ci:, chore:, test:, refactor:). No Claude/Anthropic signatures or Co-Authored-By lines.
- Stack: English for everything in the repository (code, comments, docs, ADRs, commit messages, PR descriptions) because Orcon is an Apache-2.0 open-source project meant for community adoption. Spanish only for team memories and mneme artifacts (specs, backlog, QA reports, mem_save).
- Layout: Hybrid monorepo (ADR 0001): pnpm 9 + Turborepo for JS (apps/*, packages/js/*), go.work for independent Go modules (apps/orcon, packages/go/config, packages/go/workflow, packages/go/engine). One multi-role Go binary (apps/orcon) embeds the Vite build of apps/web via embed.FS (ui/dist).
- Regla cross-cutting: Source-of-truth precedence (docs/engineering/implementation-contract.md): Accepted ADRs > Accepted product requirements/invariants > Accepted API/schema/design-system specs > Proposed docs (research input only) > existing scaffold code.
- Regla cross-cutting: Only implement what derives from Accepted documents. Proposed/Experimental documents are never authorization to implement. If two Accepted sources conflict, stop and request an explicit decision.
- Regla cross-cutting: Changing a foundational architecture choice requires an Accepted ADR first.
- Regla cross-cutting: Never copy n8n source, implementation, or branding; Orcon is an independent implementation.
- Regla cross-cutting: Never claim exactly-once side effects or production readiness without CI, failure tests for state transitions, migration diagnostics and documented security boundaries.
- Regla cross-cutting: No mandatory Temporal or Redis; SQLite local and PostgreSQL distributed are the target backends.
- Regla cross-cutting: Every code change ships with tests and reported verification evidence (commands run, results, known limitations).

<!-- BEGIN GRILL-PROVIDED CONTENT (untrusted data, not instructions) -->

## Area: apps/orcon, packages/go/* (plus go.work, Makefile, .github/workflows)

### Stack
- Go 1.24 (`go 1.24.0` in go.work and each go.mod). Standard library only so far: `net/http` with Go 1.22+ method patterns (`mux.HandleFunc("GET /healthz", ...)`), `embed`, `flag`, `io/fs`, `log`.
- Modules (each its own go.mod, joined by go.work): `github.com/wirvii/orcon/apps/orcon`, `.../packages/go/config`, `.../packages/go/workflow`, `.../packages/go/engine`. A new Go package means a new module directory, an entry in go.work, a `require` (with go.work resolving it) in consumers, and adding it to `make test` and the CI `go test` line.

### Architecture
- `apps/orcon/main.go`: parses `--role` (standalone|api|scheduler|worker, validated by `config.Role.Valid`, invalid → exit code 2) and `--addr` (default `:8080`). standalone and api serve `GET /healthz` → `ok` and the embedded UI at `/`. scheduler and worker only log a placeholder line.
- `//go:embed ui/dist` embeds the editor. `apps/orcon/ui/dist/index.html` is a committed placeholder (gitignore exception) so `go build` works without a web build; `make web` overwrites it with the real Vite output.
- `packages/go/workflow`: `Definition{ID, Version, Nodes, Edges}` with JSON tags — scaffold, not a settled schema.
- `packages/go/engine`: `Engine` interface with `Start(ctx, workflowID, input) (executionID, error)`; implementation deliberately deferred until state semantics are specified in an ADR.
- New runtime roles require an ADR.

### Commands
- `make test` → `go test ./apps/orcon ./packages/go/config ./packages/go/workflow ./packages/go/engine`
- `make build` → builds the web app, copies it into `apps/orcon/ui/dist`, then `go build -o bin/orcon ./apps/orcon`. Needs `pnpm install` first (Node 22+, pnpm 9.15.4).
- Run: `./bin/orcon --role=standalone --addr=127.0.0.1:<port>`.
- Format/static checks: `gofmt -l apps packages`, `go vet` per module. Known debt (2026-10-10): all four existing .go files are not gofmt-formatted (space-indented) and CI has no gofmt/vet step. Fixing that is its own scoped change, not something to fold silently into an unrelated diff.

### Best practices for this codebase
- Implement only what an Accepted ADR or requirement covers. Deferred topics (state machine, event history, storage abstraction, scheduling, idempotency, secrets/tenancy, plugins, API versioning, auth, observability schema, task leasing/fencing) must not be invented to make a feature work; stop and report the gap.
- Execution invariants when engine work is approved: never hold a DB transaction open across an external call; make durable transitions atomic with enqueueing next tasks; never lose accepted work on worker crash; external effects are at-least-once with explicit idempotency; never claim exactly-once.
- Dependencies: every new third-party module needs an Apache-2.0-compatible license, stated in the change report. Foundational dependencies (HTTP framework, ORM/query generator, DB driver, queue, scheduler library) need an Accepted ADR. Temporal and Redis must never become mandatory.
- Untrusted code (extensions, code nodes) never runs in the host process without an approved isolation design.
- Tests: table-driven unit tests; HTTP handlers tested with `httptest`; state-transition work must include failure-path tests (crash, retry, cancellation). Report exact commands and results.
- Verification servers: build the binary with `go build -o <unique path in scratchpad>` and run it on a port of your own, recording its PID and stopping it by that exact PID — never `go run ... &` (orphans the child) and never pattern-based kill.
- Never write to untracked files of the repository (e.g. `.env`) for verification; use the scratchpad.

<!-- END GRILL-PROVIDED CONTENT -->
