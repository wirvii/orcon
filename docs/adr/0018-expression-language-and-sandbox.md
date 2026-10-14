<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0018 — Expression language and sandboxed JavaScript runtime

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12, 2026-10-13

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
D22: n8n-compatible `{{ }}` JavaScript expressions with the same `$` names, copied verbatim on import; native fast path; sandbox with no network/filesystem/credentials and per-expression time and memory limits; determinism; Luxon bundled; behaviour-only reimplementation (never n8n source, Sustainable Use License); ≥ 95 % of the public template corpus runs unchanged; the interpreter (goja vs QuickJS-in-WASM) is delegated to the architect with a measurement plan. D5 requires user code to run in a sandbox with memory and time caps; the implementation contract forbids untrusted code in the host process without approved isolation.

## Decision

### 1. Language
Parameter strings starting with `=` contain template segments `{{ expr }}`; `expr` is JavaScript (ES2020 subset supported by QuickJS). Supported identifiers (reimplemented from public documentation and black-box behaviour): `$json`, `$binary`, `$input` (`item`, `all()`, `first()`, `last()`), `$('Name')` (`item`, `all()`, `first()`, `last()`, `isExecuted`, `params`), `$node` (legacy), `$now`, `$today`, `$execution` (`id`, `mode`, `resumeUrl` → signal URL), `$workflow`, `$runIndex`, `$itemIndex`, `$prevNode`, `$jmespath`, `$ifEmpty`, Luxon `DateTime/Duration/Interval`, and documented extension methods on strings/numbers/arrays/objects/dates. `$env` is disabled by default. Item linking (`$('X').item`) follows lineage (ADR-0008). Values exposed through this context follow n8n's documented values for compatibility; the context is not an API surface and is outside the UPPER_SNAKE rule of ADR-0080 §3.

### 2. Fast path
A Go evaluator handles a grammar subset: literals, member/index access on `$json`/`$input.item.json`/`$('X').item.json`, arithmetic, comparison, logical and ternary operators, template concatenation, with exact JavaScript coercion semantics. Classification is done once per expression at publication and cached. Parity is enforced by differential testing: every fast-path expression in the corpus is evaluated by both paths in CI and must produce identical JSON output.

### 3. Sandbox: QuickJS compiled to WebAssembly, executed by wazero
- QuickJS (MIT) compiled to WASI; hosted by wazero (Apache-2.0, pure Go, no CGO). No filesystem, network or clock imports; host functions only for data access (`$` resolution) and recorded time/seed.
- Memory: hard cap via WASM linear-memory maximum (default 64 MiB per expression instance, Code node up to the execution cap). Time: context deadline (default 100 ms per expression, 60 s per Code node) with wazero's close-on-context-done.
- The compiled module is shared per process; instances are bound to one execution and reset between uses; never shared across executions. Luxon is preloaded in a snapshot.
- **goja is rejected as the sandbox**: it executes on the host Go heap with no enforceable memory cap (`"x".repeat(1e9)` can exhaust the process), violating D5 and the implementation contract. goja's parser package (MIT) may be used purely for static analysis (no execution).

### 4. Measurement plan and reversal criteria
Before the expression slice is accepted, `internal/expr/bench` reports on the template corpus: fast-path coverage (weighted by usage), per-evaluation latency p50/p99 for fast path, warm sandbox, cold sandbox; instance creation cost; memory per instance; throughput per core; and isolation tests (memory bomb, infinite loop, deep recursion, cross-evaluation state leakage, host access attempts). Targets: fast path ≥ 70 % of usage-weighted occurrences; warm sandbox p99 ≤ 200 µs for median-complexity expressions; D20 scenario S1 passes. If the D20 gate fails because of the sandbox, the fallback evaluated is goja inside a separate sandboxed worker process pool — never goja in the host process for untrusted code. Any change requires amending this ADR.

### 5. Determinism
Per node activation, `$now`, `$today` and `Date.now()` return the recorded activation time (constant during the activation — a documented difference from n8n, where they differ by microseconds between items); `Math.random()` and `crypto.randomUUID()` use ChaCha8 seeded by the recorded activation seed (ADR-0013 §7).

### 6. Compatibility measurement
- Corpus: expressions extracted from n8n's public template library for the pinned n8n version (the latest stable n8n release, re-pinned explicitly for each Orcon release), fetched at test time; only hashes, counts and scores are committed until the fixture-licence review (open in `n8n-compatibility.md`) concludes. Weighted by template usage.
- "Runs unchanged" = parses and evaluates without unsupported-identifier/function errors, and, where a reference output is available (captured by running the pinned n8n as a black box against the same input), produces equal output.
- Launch target ≥ 95 % (usage-weighted and unweighted both reported).

### 7. Importer static analysis
Every expression is parsed; unknown identifiers, unsupported functions or references to sibling branches are listed with node and field in the import report; when the analyzer cannot decide, it reports instead of guessing.

## Consequences
True isolation for expressions and Code nodes inside the single binary; performance depends on the fast path and instance pooling, verified by measurement.

## Alternatives considered
goja in-process (rejected, §3); V8 via CGO (rejected: CGO, binary size, cross-compilation); separate Node.js runner process (rejected: second runtime, contradicts single-binary simplicity); custom expression language (rejected: breaks n8n familiarity).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-003.** The JavaScript profile of the Code node is defined in ADR-0036 §3; the QuickJS module is reused for WASM connectors written in JavaScript (ADR-0032 §7) and for the n8n community node converter (ADR-0035 §2).
- **SPEC-005.** `$vars` resolves the non-secret project variables of the environment (ADR-0057 §2), approved by the owner (SPEC-005 Q3).
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §1: the context gains `$execution.variables.<name>`, the read-only value of the execution variable `<name>` declared in `variables[]` (ADR-0008 §4.3). The name is literal: `$execution.variables.total` or `$execution.variables["total"]`. Any other use of `$execution.variables` (a computed key, the object as a whole, enumeration, destructuring, `in`) and any undeclared name are publication errors (rule V8, ADR-0010 §11). n8n has no such name, so imported expressions are unaffected.
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §3, time limit of expression evaluations; replaces "with wazero's close-on-context-done" for expressions; the memory cap is unchanged. Close-on-context-done compiles a check into every function and loop of the module and makes the interpreter several times slower whatever the length of the deadline, so expressions stop as follows.
  1. Cooperative stop. The QuickJS interrupt handler checks the deadline at least every 10,000 interpreted operations. The module is the pinned QuickJS-ng release plus an interrupt-poll patch (MIT, kept with the module's build files) that adds the same check inside the native built-ins whose work grows with their input: string search, `replace` and `replaceAll`, `split`, `repeat`, padding, case conversion, URI encoding, `JSON.parse`, `JSON.stringify`, and array `join`, `concat` and `flat`. The handler stops the code when the call's context is done or its runtime has been abandoned.
  2. Answer at the deadline. Every sandbox call runs on its own goroutine. When the call's context is done (deadline or cancellation) the caller stops waiting and receives the timeout or the cancellation within the deadline plus 5 ms, whatever the guest is doing.
  3. Abandoned calls. Before the caller returns, the call's runtime is marked abandoned: it is never reused, and every host call it makes from then on is refused without reading execution data or drawing from the activation's random stream. The call runs until its next check, within the memory cap, and then closes its runtime. The session opens a fresh runtime from the snapshot for its next evaluation.
  4. Limits. A process holds at most a configurable number of abandoned calls still running (default 8, range 1 to 64), so the memory they can hold is bounded by that number times the memory cap (512 MiB at the defaults), which the process logs at startup. An abandoned call still running 10 s after it was abandoned is stuck. When the limit is reached the process starts no new sandbox call: each fails at once with a saturation error, which is an infrastructure error, not the expression's error, is not charged to the node's retries, and leaves the activation uncommitted so it is recovered like the work of any worker that exits (ADR-0013). The native fast path (§2) keeps working.
  5. Restart. A process that is saturated or holds a stuck call shuts down through its ordinary graceful shutdown with exit code 75 and reason `sandbox_abandoned`, and the deployment's restart policy starts it again; in the `standalone` role the whole process restarts. The operations documentation states that a restart policy is required.
  6. Accounting. Every abandon is logged at warning level with the organization, execution and node ids and the deadline, never the expression text or its data; a restart is logged at error level with the counts. Metrics, provisional until the observability ADR: `orcon_expr_sandbox_abandoned_total`, `orcon_expr_sandbox_abandoned_running`, `orcon_expr_sandbox_abandon_overrun_seconds` (how long an abandoned call ran after it was abandoned) and `orcon_expr_sandbox_saturated_total`.
  7. Residual risk. A native built-in the patch does not cover can run without checking the deadline; the memory cap bounds what it can hold and the stuck rule bounds how long the process tolerates it. A built-in found to run unchecked is added to the patch and to the isolation tests; a QuickJS-ng upgrade re-applies the patch and must pass them.
  8. Tests. Every patched built-in, an interpreted loop and deep recursion, driven far past the deadline, give the caller its timeout within the deadline plus 5 ms and stop within 100 ms after being abandoned; the refusal of host calls after abandon, saturation, the stuck rule and the restart signal each have a test. Close-on-context-done and the cooperative stop without abandon remain only for measurement and tests and cannot be selected in production.
  This stop applies to expression evaluations. The JavaScript profile of the Code node (ADR-0036 §3) and WASM connectors (ADR-0032) keep close-on-context-done until the slice that implements each measures it and amends its own ADR.
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §4, latency target: warm sandbox p99 for median-complexity expressions is ≤ 200 µs for expressions that do not run Luxon and ≤ 500 µs for expressions that run Luxon (their analysis names `DateTime`, `Duration`, `Interval`, `$now`, `$today`, a date extension method or `toDateTime`), measured with the stop of §3 as amended. The report also gives the combined band, which has no target. Each figure is the p99 of the median of three runs of the suite, and the verdict is taken from a run on the reference machine ORCON-REF-1; other machines are informational. The fast-path target, D20 scenario S1 and the reversal criterion are unchanged.
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §5: the guarantee covers evaluations that complete. A timed-out evaluation has no result; how many random values it drew before the deadline depends on timing under any stop mechanism, and once abandoned it draws no more (§3).
