<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0036 — Code node: JavaScript, Python and Go in WebAssembly sandboxes

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K6: V1 Code node languages are JavaScript (same sandboxed interpreter as expressions, ADR-0018, with the n8n-compatible API and inclusion in the compatibility suite), Python (sandboxed CPython in WebAssembly with a warm pool to hide startup) and Go (owner addition: an interpreter, yaegi, running inside the WebAssembly sandbox, with a Go-style helper API; later an optional compiled mode with identical source). All: no network, filesystem or credentials unless passed; time and memory caps; failure isolated to that execution. Libraries: curated, reviewed set; admins may add others at their own risk; the import report flags unavailable libraries including Python C-extension packages; the editor flags unsupported Go features while typing. yaegi's licence is Apache-2.0 (verified). Public evidence shows yaegi compiled to WebAssembly in browsers (`js/wasm`); no evidence was found for `wasip1` under wazero or its performance.

## Decision

### 1. Node `orcon.code@1`
Parameters: `language: JAVASCRIPT | PYTHON | GO`, `mode: ALL_ITEMS | EACH_ITEM`, `source`, `libraries[]` (pinned like connectors, ADR-0034), `timeout` (default 60 s, instance max), `memoryMiB` (default 256, ≤ execution cap). Purity `PURE` (recomputable; time and randomness recorded, ADR-0013 §7). `$getWorkflowStaticData` (JS) and its Python/Go equivalents are flow-data internal effects (ADR-0008 §4.4) with documented differences: writes are committed at the node's commit point, not at the end of the execution (owner decision, SPEC-003 Q14).

### 2. Common sandbox rules
No network, no writable filesystem (libraries mounted as a read-only in-memory image), no environment, no credentials; console/print output → execution log (redacted, 1 MiB cap); output size within execution limits; instances never shared across executions; failure → the node's error rules (ADR-0011 §3).

### 3. JavaScript
The ADR-0018 QuickJS-WASM module with a Code profile (time and memory above). API reimplemented from public documentation: `$input.all()/first()/last()/item`, `$json`, `$binary` and a binary-buffer helper (streamed within the memory cap), `$('Name')`, `$node`, `$execution`, `$workflow`, `$now`, `$today`, `$jmespath`, `$getWorkflowStaticData`, legacy `items`/`$items()`; return-value normalisation per the documented n8n rules; Promises and `async/await` driven by the host job loop; `require` limited to installed libraries. Unavailable in V1 in every language: network helpers (`this.helpers.httpRequest`, `fetch` and equivalents) and Node.js built-ins (`fs`, `child_process`, `process`); the importer reports them and suggests the HTTP node (owner decision, SPEC-003 Q6). A later version may add an explicit per-node network permission limited to approved domains and enforced by the egress client (ADR-0029 §8); that requires amending this ADR. Included in the D22 compatibility suite.

### 4. Python
CPython (PSF-2.0) compiled for WASI with wasi-sdk, pre-initialised with Wizer (interpreter started, Orcon helper module and curated pure-Python libraries imported) into a snapshot module; hosted by wazero. API names follow n8n's documented Python Code node (`_input.all()`, `_input.first()`, `_input.item`, `_json`, `_items` legacy, `_node`, `_now`, `_execution`, `_workflow`), reimplemented; data crosses as JSON (plain dicts/lists). Warm pool per worker: N fresh instances from the snapshot (default `min(4, CPU)`, memory-budget aware via ADR-0016 admission control), single-use, refilled in the background; cold instantiation when empty. Libraries: pure-Python wheels in the read-only image; C-extension packages are unavailable unless compiled into a curated runtime build; the importer reports `import` statements of unavailable modules.
Distribution (owner decision, SPEC-003 Q7): the runtime is a separate official signed `RUNTIME` package `orcon.runtime.python`, preinstalled in Orcon's container images and installable offline (ADR-0033 §8); it is not embedded in the base binary. Without it, Python is shown as unavailable in the editor and in import reports.
Measurement plan (report required before acceptance): module size, cold start, snapshot instantiation, warm acquisition p50/p99, memory per instance, throughput on a corpus of representative programs. Targets: warm acquisition p99 ≤ 5 ms; cold start ≤ 1 s; baseline memory per instance ≤ 64 MiB.

### 5. Go
- Runtime: yaegi compiled for `GOOS=wasip1 GOARCH=wasm` together with the Orcon helper package and curated standard-library symbol tables (generated with `yaegi extract`), excluding `unsafe`, `syscall`, `os/exec`, `net`, `plugin` and filesystem-writing APIs; executed by wazero.
- Program shape: `func Run(in orcon.Input) ([]orcon.Item, error)` (`ALL_ITEMS`) or `func RunItem(it orcon.Item) (orcon.Item, error)` (`EACH_ITEM`). Helper API: `in.All()`, `in.First()`, `in.Last()`, `it.JSON` (`map[string]any`), `orcon.Node("Name").All()`, `orcon.Now()`, `orcon.Execution()`.
- Editor diagnostics: an API endpoint parses and type-checks the source with `go/parser` and `go/types` (standard library) against an importer limited to available packages, plus a list of constructs known to be unsupported by the pinned yaegi version (kept accurate by tests); results are shown while typing (debounced).
- Additional libraries: pure-Go source modules interpreted by yaegi from the read-only image (no cgo, no `unsafe`).
- **Prototype gate (first step of the implementing slice)**: build the module and measure: module size (≤ 64 MiB uncompressed); compilation-cache instantiation p99 (≤ 50 ms); warm-pool acquisition p99 (≤ 5 ms); baseline memory per instance (≤ 64 MiB); ten representative transformations over 1,000 items (≤ 3× the time of the equivalent JavaScript Code node); the isolation suite (memory bomb, infinite loop, goroutine bomb, deep recursion, host access attempts, cross-execution leakage) passes. Results published in `docs/benchmarks/code-node-go.md`.
- **If the gate fails, fallback F1 applies (owner decision, SPEC-003 Q13; no further decision needed)**: yaegi runs natively in a pool of isolated helper subprocesses — the `orcon` binary re-executed as a hidden sandbox helper, with a separate process per evaluation slot, resource limits (memory, CPU time, process count), no network, no writable filesystem, an empty environment and, on Linux, seccomp and landlock filters. Isolation on Linux is strong; on macOS and Windows it is weaker (no equivalent syscall filtering): this is documented and the admin sees a visible warning in settings and in the Code node when the Go language is used on those platforms. The same API, editor diagnostics and isolation suite apply; the measurements of the F1 implementation are published in the same report. The decision between WASM and F1 is recorded in `docs/benchmarks/code-node-go.md`.
- yaegi in the host process is never an option.
- Later compiled mode: identical source compiled to `wasip1` at publish time when a toolchain is available; same API; requires an ADR amendment.

### 6. Curated libraries
Catalog package kind `LIBRARY` (`language`, `name`, `version`, `license`, `digest`, seal), reviewed like connectors and recorded in the instance table `code_library` (ADR-0029 §8). Initial sets per language are chosen by measured frequency of `require`/`import` statements in the template corpus Code nodes (ADR-0037). Admins may allow uncurated libraries (`codeNode.allowUncuratedLibraries`, default false; shown `UNREVIEWED`). JS: bundled pure-JS modules; Python: pure wheels; Go: pure-Go source modules.

### 7. Import
`n8n-nodes-base.code` (JavaScript, Python) → `orcon.code@1`; legacy Function / Function Item → `JAVASCRIPT` with mode `ALL_ITEMS` / `EACH_ITEM` and the `items` shim. Static analysis lists unsupported APIs and unavailable libraries with node and line.

## Consequences
Three languages with one isolation model where feasible; JavaScript shares the expression runtime; Python and Go costs are measured before commitment; the Go fallback is decided in advance, so a failed prototype does not stall the slice.

## Alternatives considered
Pyodide (rejected: Emscripten/browser target, not WASI); Python in a native subprocess (rejected: weaker isolation, second runtime); Python runtime embedded in the base binary (rejected, SPEC-003 Q7: inflates every install by tens of MB); yaegi in the host process (rejected: untrusted code in host, no memory cap); compiled-only Go as fallback F2 (rejected as fallback, SPEC-003 Q13: needs a Go toolchain on the server); network in the Code node in V1 (rejected, SPEC-003 Q6); goja for the JavaScript Code node (rejected by ADR-0018).
