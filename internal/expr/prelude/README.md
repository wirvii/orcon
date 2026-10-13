<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Expression prelude

The scripts every expression runtime loads, in this order (`expr.Prelude`):

| Script | Purpose |
|---|---|
| `intl.js` | A minimal English `Intl` (QuickJS has none) that Luxon needs for time zones and date formatting; first script, it parks the host function in `globalThis.__orcon` |
| `luxon.min.js` | Luxon 3.7.2 (MIT), the global build, vendored unmodified; `vendor-luxon.sh` refreshes it from the pinned npm tarball after checking its SHA-256 (`LICENSE.luxon` is its licence text) |
| `runtime.js` | `__orcon_run`, the entry point; the n8n context (`$json`, `$('Node')`, `$now`, ...), lazy parsing of the request sections, deterministic `Math.random` and `crypto` |
| `jmespath.js` | `$jmespath`, a JMESPath implementation written from the language specification |
| `extensions.js` | The n8n extension methods (strings, numbers, arrays, objects, dates) |
| `harden.js` | Last: removes the private namespace and freezes every reachable built-in, so no expression can change the runtime |

The protocol between these scripts and `internal/expr` is described in `docs/reference/expressions.md`; the host operations are the `Op…` constants of `internal/expr/sandbox.go`.
