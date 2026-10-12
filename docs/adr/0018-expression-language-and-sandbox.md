<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0018 — Expression language and sandboxed JavaScript runtime

Status: **Accepted** | Date: 2026-10-11

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
