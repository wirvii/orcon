<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Expressions

Status: Accepted (implements ADR-0018; SPEC-001 cut C5)

How the expression engine of `internal/expr` evaluates the `{{ }}` segments of a parameter value, what it guarantees, and where it differs from n8n. The measurement that backs the design is `internal/expr/bench/REPORT.md`.

## Language

A parameter value that starts with `=` is a template (ADR-0007 section 3). Its `{{ }}` segments hold JavaScript (the ES2020 subset QuickJS implements). The closing `}}` is found by a light JavaScript scan, so `{{ {a: {b: 1}}.a.b }}` and `{{ "}}" }}` are one segment each; a `{{` without a closing `}}` is literal text.

- A template that is exactly one segment (`={{ $json.a }}`) returns the typed JSON value. Any other template returns a string, each segment converted with the JavaScript string conversion (`undefined` becomes `"undefined"`, an object becomes `"[object Object]"`).
- A trailing `;` after the expression is accepted, as in n8n.
- A result that is `undefined` is reported as such (`Result.Undefined`); NaN and the infinities become `null`, as `JSON.stringify` does.

## The context

The identifiers keep the n8n names and values (ADR-0018 section 1). They are not an API surface.

| Name | Value |
|---|---|
| `$json`, `$binary` | data and binary metadata of the current item |
| `$input` | `item`, `all()`, `first()`, `last()`, `itemMatching(i)`, `params` |
| `$('Name')` | `item` (the paired item, by lineage), `first()`, `last()`, `all()`, `itemMatching(i)`, `isExecuted`, `params` |
| `$node['Name']` | legacy: `json` (also `data`), `binary`, `parameter`, `runIndex` of the paired item |
| `$items(name?)` | legacy: the items of a node, or of the input |
| `$now`, `$today` | Luxon `DateTime` of the activation time, in the flow's time zone |
| `$execution`, `$workflow`, `$prevNode` | `id`/`mode`/`resumeUrl`/`resumeFormUrl`; `id`/`name`/`active`; `name`/`outputIndex`/`runIndex` |
| `$runIndex`, `$itemIndex`, `$position` | counters |
| `$vars`, `$parameter` | project variables; parameters of the node being evaluated |
| `$jmespath(data, query)` | full JMESPath |
| `$ifEmpty`, `$if`, `$min`, `$max` | helpers |
| `DateTime`, `Duration`, `Interval` | Luxon 3.7.2, unmodified |
| `$env` | disabled: any access throws `access to env vars denied` (ADR-0018 section 1) |

`$fromAI` belongs to the agent specification (SPEC-006) and is not defined here: the analysis reports it as owned by a later specification.

The extension methods of n8n are implemented from their public documentation: on strings (`base64Encode`, `base64Decode`, `extractDomain`, `extractEmail`, `extractUrl`, `extractUrlPath`, `hash`, `isDomain`, `isEmail`, `isEmpty`, `isNotEmpty`, `isNumeric`, `isUrl`, `parseJson`, `quote`, `removeMarkdown`, `removeTags`, `replaceSpecialChars`, `toBoolean`, `toDate`, `toDateTime`, `toDecimalNumber`, `toFloat`, `toInt`, `toSentenceCase`, `toSnakeCase`, `toTitleCase`, `toWholeNumber`, `urlDecode`, `urlEncode`), numbers (`abs`, `ceil`, `floor`, `format`, `isEven`, `isOdd`, `round`, `toBoolean`, `toDateTime`), booleans (`toInt`), arrays (`append`, `average`, `chunk`, `compact`, `difference`, `first`, `intersection`, `isEmpty`, `isNotEmpty`, `last`, `max`, `merge`, `min`, `pluck`, `randomItem`, `removeDuplicates`, `renameKeys`, `smartJoin`, `sum`, `toJsonString`, `union`, `unique`), objects (`compact`, `hasField`, `isEmpty`, `isNotEmpty`, `keepFieldsContaining`, `merge`, `removeField`, `removeFieldsContaining`, `toJsonString`, `urlEncode`) and dates (`beginningOf`, `diffTo`, `endOfMonth`, `extract`, `format`, `isDst`, `isInLast`, `isWeekend`, `minus`, `plus`, `toDateTime`). A method the language already has is never replaced.

## Two engines, one result

An expression is classified once, when the flow version is compiled (`Evaluator.Compile`):

- **Fast path.** A Go evaluator for literals, member and index access on the item data (`$json`, `$input.item.json`, `$input.first().json`, `$('X').item.json`, `$node['X'].json`), arithmetic (`+ - * / %`), comparison, logical, nullish and conditional operators, `typeof` and `!`, with the coercion rules of the language. It never answers differently from the interpreter: when a value would need behaviour it does not reproduce (an object where a primitive is needed, a property that lives on a prototype, a read of `undefined`, a node that did not run) it hands the expression to the sandbox. Parity is enforced by `TestFastPathParity`, which evaluates every operator over every pair of a set of awkward values on both paths and compares the canonical JSON, and by `TestExprCorpusParity` (under `make expr-compat`), which does the same for every fast-path expression of the template corpus against synthetic data.
- **Sandbox.** Everything else runs in QuickJS compiled to WebAssembly and executed by wazero (the `internal/adapter/wazero` adapter implements the `expr.Sandbox` port).

## The sandbox

- **No host.** The module has no filesystem, network, process or clock of its own. Its only imports are `orcon_v1.host_call` (the data of the context and recorded randomness), `orcon_v1.interrupt` and five WASI functions answered by the adapter; the clock is the recorded activation time.
- **Memory.** The WebAssembly memory is capped (default 64 MiB per runtime) and the JavaScript heap limit sits 8 MiB below it, so a memory bomb raises a catchable `InternalError` and the evaluation fails with `ErrMemory`.
- **Time.** Each call runs under the context deadline (default 100 ms): `ErrTimeout`. See "Time limit modes".
- **Stack.** The JavaScript stack is capped (512 KiB); runaway recursion throws a `RangeError` that is also reported as `ErrStack`.
- **Isolation between executions.** A runtime serves one execution (`Session`) and is closed with it; it is never shared. After any sandbox failure the runtime is discarded and the next evaluation of the session starts from a pristine one. Within a runtime, the prelude freezes every built-in and the global object, so an expression cannot leave state behind (`globalThis.x = 1`, `Array.prototype.y = …`, `Math.random = …` are ignored), and the shared helpers are frozen too. `TestSandboxIsolation` checks all of this in both time-limit modes.
- **Snapshots.** The prelude (Luxon, the context, the extension methods) is loaded once per configuration and the resulting memory is kept; a new runtime copies it instead of loading the scripts again, which takes well under a millisecond.

### Request protocol

A call carries a request string: a JSON header (`e` code, `m` mode `v`alue or `s`tring, `i`/`r` indexes, `n` activation time in ms, `z` zone, optional `x`/`w`/`p` execution, workflow and previous-node headers, `l` lenient) followed by four sections separated by `U+0001`: `$json`, `$binary`, `$vars`, `$parameter`, each as JSON text, empty when the code does not read it. Parsing JSON is the largest cost of a sandbox call, so `internal/expr` analyzes the code once and sends only what it reads; when the code reads `$json` only through static member chains (`$json.user.address.city`) the section holds just those paths. The answer is `{"v": value}`, `{"u": 1}` (undefined), `{"s": text}` or `{"x": {n, m, c?}}` (an error thrown by the expression; `c` marks a compile-time syntax error). During the call the runtime may ask the host for the input items, other nodes' outputs, paired items, random numbers, zone conversions and hashes (the `Op…` constants).

### Time limit modes

ADR-0018 section 3 prescribes enforcing the deadline with wazero's close-on-context-done. wazero implements it by compiling a check into every function and loop of the module, and the QuickJS interpreter pays it on every bytecode operation, so it runs several times slower. The adapter therefore has two modes (`wazero.WithMode`):

- `ModeHardLimit` (default): the ADR as written.
- `ModeCooperativeLimit`: the deadline is enforced by the QuickJS interrupt handler, which the interpreter polls every ten thousand operations and inside regular expressions and long built-ins. Code that never polls (for example `JSON.stringify` of a 100000-deep array) is bounded only by the memory cap and can outlive the deadline; `TestSandboxIsolation` records a case that takes about 2.7 s against a 150 ms limit.

The measurement report compares them. Choosing between them is the owner's decision (an amendment of ADR-0018 section 3).

## Determinism

Per node activation (`expr.Activation`): `$now`, `$today`, `Date.now()` and `new Date()` return the recorded activation time and do not move during the activation; `Math.random()` and `crypto.randomUUID()` draw from a ChaCha8 stream seeded with the recorded seed (`rand.NewChaCha8`). Rebuilding the activation from the same time and seed and evaluating the same expressions in the same order gives identical results (`TestDeterminism`); the order of draws is part of the record.

## Differences from n8n

- `$now` is constant during an activation (n8n's differs by microseconds between items).
- `Intl` is a minimal English subset: `toLocaleString`/`Intl.DateTimeFormat` accept English locales only and throw a `RangeError` for another locale rather than formatting wrongly; Luxon's localized formats (`DATE_HUGE`, `toLocaleString`) work in English. Time zone names print as `GMT±h`, `UTC`.
- Built-ins are frozen: assignments to them are ignored or throw. Class code that assigns `this.name` in a subclass of `Error` fails ("name is read-only").
- The extension methods are installed as non-enumerable properties of the prototypes. Reading (without calling) a name that is an extension method, for example `$json.compact` on an object without that field, yields the method instead of `undefined`. n8n rewrites calls instead and leaves property reads alone. The one conflict with a library member that matters is Luxon's `isWeekend` getter: calls of `.isWeekend()` are renamed when the expression is prepared, property reads stay Luxon's.
- No `Promise`/async, `URL`, `Buffer`, `process`, `require`, `fetch`, timers or `console`.
- A JSON number that does not fit a double loses precision inside the sandbox, as in JavaScript; the fast path keeps the text. Their canonical JSON is the same.
- Object members come back from the sandbox in JavaScript order (integer-like keys first); the fast path keeps insertion order. Orcon treats member order as insignificant.

## Static analysis

`expr.Analyze` parses an expression with the goja parser (parser only; goja never executes anything, ADR-0018 section 3) and reports its free identifiers, the node references (`$('X')`, `$node["X"]`, `$items("X")`, with `.isExecuted` guards), the method names it calls, the property paths it reads from `$json` and anything it cannot vouch for (syntax errors, unknown identifiers or methods, node names computed at run time). The n8n importer lists these findings with node and field (ADR-0018 section 7). Rule V13 of the flow validator uses it (see `orcon-flow-1.md`).

The lists of known names are generated from the runtime itself: `go test ./internal/adapter/wazero -run TestGeneratedNames -update` rewrites `internal/expr/names_gen.go` after the prelude changes, and `TestKnownIdentifiers` fails when the lists of identifiers drift from what the sandbox defines.

## Third-party components

| Component | Version | Licence | Where |
|---|---|---|---|
| QuickJS-ng | 0.17.0 | MIT | compiled into `internal/adapter/wazero/assets/quickjs.wasm` |
| Luxon | 3.7.2 | MIT | `internal/expr/prelude/luxon.min.js`, unmodified |
| wazero | 1.11.0 | Apache-2.0 | Go module, `internal/adapter/wazero` only |
| goja (parser packages only) | commit dc0ad5d | MIT | Go module, static analysis only |
| wasi-sdk | 34.0 | Apache-2.0 with LLVM exception | build toolchain of the module, not distributed |

`assets/build.sh` and `prelude/vendor-luxon.sh` rebuild or refresh the two embedded artifacts from pinned, checksummed downloads; see the READMEs next to them.
