<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Expression runtime module

`quickjs.wasm` is the WebAssembly module the expression sandbox runs (ADR-0018 section 3): QuickJS-ng compiled to a WASI reactor, plus `orcon_qjs.c`, a small C shim written for Orcon that exposes `orcon_init`, `orcon_load`, `orcon_bind`, `orcon_eval`, `orcon_out_ptr`, `orcon_out_len`, `orcon_memory_used`, `orcon_alloc` and `orcon_free`, and imports `orcon_v1.host_call` and `orcon_v1.interrupt`. The module uses no std/os/module-loader code of QuickJS and no filesystem, network or clock.

The binary is committed, as the reviewed output of `build.sh`; CI does not rebuild it. Rebuilding with the same toolchain archive gives the same bytes (SHA-256 of `quickjs.wasm`: `a0b8968a413120f02a46a14df5c514810bc4de1dad9c4559f15b864c3f0914b7`, checked with the arm64-macos archive).

## Rebuilding

```sh
sh internal/adapter/wazero/assets/build.sh
```

The script downloads wasi-sdk 34.0, QuickJS-ng 0.17.0 and nothing else, checks each download against the SHA-256 digest written in the script, applies `quickjs-interrupt-poll.patch` to `quickjs.c`, compiles `orcon_qjs.c` with the QuickJS sources (`quickjs.c`, `libregexp.c`, `libunicode.c`, `dtoa.c`; not `quickjs-libc.c`) and writes `quickjs.wasm` and `LICENSE.quickjs`. Set `WASI_SDK_PLATFORM` and `WASI_SDK_SHA256` to use the toolchain archive of another build machine; the module itself does not depend on the machine.

| Input | Version | Licence | SHA-256 |
|---|---|---|---|
| wasi-sdk (arm64-macos) | 34.0 | Apache-2.0 with LLVM exception | `9c59398106b417f8f14913380fdf0097a8cc0ff4af9eb3ce0065a859e88d49e9` |
| QuickJS-ng source | 0.17.0 | MIT | `559bc4c420475e55c7ab4510adbc562f55d7524d75e8e89d79ce4bb02f5687d9` |

## Files

- `quickjs.wasm`: the module (MIT, QuickJS-ng; `LICENSE.quickjs` is its licence text).
- `quickjs-interrupt-poll.patch`: the Orcon change to QuickJS-ng 0.17.0 (a unified diff of `quickjs.c`). It adds `js_poll_work`, which charges the work of a native loop to the interrupt counter and calls the interrupt handler when it runs out, and polls it inside the built-ins whose work grows with the input: string search (`indexOf`, `lastIndexOf`, `includes`, the search of `replace` and `split`), `replace`/`replaceAll`, `split`, `repeat`, `padStart`/`padEnd`, `JSON.parse`, `JSON.stringify`, `Array.prototype.join`, `concat`, `flat`, `toLowerCase`/`toUpperCase` and URI encoding. Without an interrupt handler installed (the hard-limit mode) the polls cost one decrement per step. Upgrading QuickJS-ng means re-applying the patch and passing `TestSandboxIsolationAbandon`; a built-in found to run without checking the deadline is added to the patch and to that test (SPEC-001 amendment 10, B.1 point 8). The patched module is "QuickJS-ng 0.17.0 + interrupt-poll patch, MIT"; the patch is versioned with the module and rebuilt with it.
- `orcon_qjs.c`: the shim (Apache-2.0, The Orcon Authors).
- `build.sh`: the build.
