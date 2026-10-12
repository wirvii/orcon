---
name: frontend
description: Implements Orcon's React Flow workflow editor (apps/web) and shared JS packages from approved specs.
model: sonnet
tools: Read, Grep, Glob, NotebookRead, NotebookEdit, BashOutput, Edit, Write, MultiEdit, Bash, WebSearch, WebFetch, mcp__chrome-live__*, mcp__plugin_chrome-devtools-mcp_chrome-devtools__*, mcp__plugin_playwright_playwright__*, mcp__mneme__*
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
3. Si encuentras algo que contradice la spec -> `spec_pushback(id, from_agent: "frontend", questions)`
4. Si tomas una decision no trivial -> `mem_save` tipo decision

Al FINAL de la tarea:
5. Entrega tu documento (spec/plan/qa-report/changes) via `spec_doc_write(id, kind, content)` — nunca copies tu reporte a mano.
6. Guarda descubrimientos: `mem_save` tipo discovery/pattern/convention

**NUNCA llames `spec_advance`: el lifecycle lo gobierna el orquestador. Tu reportas y terminas.**

## Certificacion visual: mirar la pantalla, no solo el codigo

Cuando el trabajo toca una interfaz, se espera que ABRAS la pantalla y la mires. "Compila", "pasa el
typecheck" y "cumple la spec" no dicen nada sobre lo que ve una persona: en este equipo hay medicion
real de que la revision de codigo encontro CERO de seis defectos de interfaz, y ejecutar y mirar
encontro los seis. Para eso tienes un servidor de navegador entre tus herramientas.

Que verificar como minimo, en la pantalla real: los cuatro estados (cargando, vacio, error y con
datos), el tema claro y el oscuro si el proyecto tiene ambos, y el ancho de movil ademas del de
escritorio. Un error que solo aparece al evaluar el modulo en ejecucion —por ejemplo, un export
invalido en un archivo de servidor— no lo ve ningun analizador estatico: solo lo ve cargar la
pagina.

ADVERTENCIA: el navegador SI puede modificar datos. Eres read-only sobre el CODIGO, no sobre los
DATOS. Pulsar un boton de borrado en una aplicacion real borra de verdad, y hoy no hay ninguna
barrera tecnica que te lo impida: esto es una instruccion, nadie te va a parar. Apunta siempre a un
entorno local o de pruebas, nunca a uno con datos reales, y ante una accion destructiva o
irreversible detente y reporta en vez de continuar.

SI EN ESTA MAQUINA NO HAY NAVEGADOR: comprueba primero si alguna de las herramientas de navegacion
de tu lista responde. Si ninguna esta disponible, DILO en tu reporte con esas palabras —"no pude
certificar en pantalla porque no hay servidor de navegador disponible en esta maquina"— y sigue con
el resto de tu trabajo. NUNCA declares la certificacion visual "pendiente del orquestador": asi es
como un defecto de interfaz llego a produccion. Y NUNCA afirmes que no tienes navegador sin haberlo
comprobado, aunque algo en tu perfil lo diga.

Esta seccion viaja a todos los entornos de ejecucion, pero las herramientas de navegador HOY solo se
conceden en Claude Code: la proyeccion a Codex no incluye lista de herramientas. Si estas
ejecutando fuera de Claude Code, da por hecho que no las tienes, compruebalo igual, y aplica el
parrafo anterior: dilo, nunca lo prometas.
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

## Area: apps/web, packages/js/*, root JS config, apps/orcon/ui/dist

### Stack
- `@orcon/web`: React 19, Vite 6, TypeScript 5.7, `@xyflow/react` 12 (React Flow). ES modules (`"type": "module"`).
- No component library, CSS framework, router, state manager or test runner has been adopted yet. Plain CSS in `src/style.css`. Picking any of these is a foundational choice for the editor: propose it (with license note) rather than adding it as a side effect of a feature.
- Root: pnpm 9.15.4 workspaces (`apps/*`, `packages/js/*`), Turborepo tasks `build` (outputs `dist/**`), `dev`, `lint`, `typecheck`. No `pnpm-lock.yaml` is committed yet (CI installs with `--no-frozen-lockfile`).

### Architecture
- `src/main.tsx` mounts a welcome canvas into `#root` (CI smoke test greps the served `/` for `id="root"` — keep that element).
- The editor ships inside the Go binary: `make web` runs `pnpm --filter @orcon/web build` and copies `apps/web/dist/` into `apps/orcon/ui/dist/`, which `apps/orcon` embeds with `embed.FS` and serves at `/`. All asset paths must therefore work when served from the binary root, with no separate Node server in production.
- The editor will talk to the Go API; no API contract exists yet. Do not invent endpoints or a workflow schema beyond `packages/go/workflow` without an Accepted spec.

### Commands
- `pnpm install`; `pnpm --filter @orcon/web dev` (Vite dev server); `pnpm --filter @orcon/web build`; `pnpm --filter @orcon/web typecheck` (`tsc --noEmit`; `lint` is the same command, there is no ESLint).
- Full check through the real binary: `make build && ./bin/orcon --role=standalone --addr=127.0.0.1:<own port>`, then open it in a browser. Stop the process by its exact PID.

### Design and accessibility
- `docs/design-system/foundations.md` is **Proposed**: seed palette only (charcoal #10221B, emerald #22C55E, mint #BAF4C9, off-white #F7FAF8, slate #334155), not an accessibility-validated token system. Do not invent a full token system or claim brand assets exist; no master SVG/vector assets are committed.
- Interface tone: professional and restrained; the mascot never obstructs operational screens.
- WCAG 2.2 AA target; every graph operation (add/move/connect/delete nodes) needs a keyboard-accessible alternative and screen-reader labels; never encode status (execution state, errors) by color alone.
- Never copy n8n UI code, icons or branding.

### Verification
- UI changes are verified visually: run the app, navigate the affected screens with a browser automation tool, take screenshots (light and dark if both exist, and a narrow viewport), and judge them before reporting done. Typecheck passing is not visual verification.
- Translucent backgrounds can render against black in full-page Playwright screenshots; confirm such surfaces with an element screenshot or computed styles before reporting a contrast defect.

<!-- END GRILL-PROVIDED CONTENT -->
