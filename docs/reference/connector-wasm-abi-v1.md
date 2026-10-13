<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# WebAssembly connector ABI v1

Status: **Accepted** | Source: ADR-0032 (accepted 2026-10-12 via SPEC-003)

Reference for authors of third-party WebAssembly connectors. The text below is taken from ADR-0032 §1–§4, §6 and §8, which are normative; if this page and the ADR disagree, the ADR wins. Packaging, manifest and trust rules are in ADR-0029 and ADR-0033; the SDKs are described in ADR-0032 §7.

## Module exports (ADR-0032 §1)
A core WebAssembly module (wasm32) importing only module `orcon_v1` and the allowed WASI preview1 subset (§3). It exports:
- `orcon_abi_version() -> i32` (returns 1),
- `orcon_alloc(size: i32) -> i32`, `orcon_free(ptr: i32, size: i32)`,
- `orcon_describe() -> i64` (packed `ptr<<32 | len` of a JSON document that must equal the package manifest's node/operation list),
- `orcon_invoke(ptr: i32, len: i32) -> i64` (packed result).
Requests and responses are UTF-8 JSON in ABI v1 (owner decision, SPEC-003 Q12). Request: `{ op, node, operation?, parameters, items?: {inline | pageRef}, intent?, cursor?, request? }` where `op ∈ { read, prepare, dispatch, reconcile, trigger.verify, trigger.handshake, trigger.decode, poll.initialCursor, poll.poll, register, deregister, list, renew, credential.test }` (operation identifiers). Response: `{ items? | intents? | outcome? | cursor? | ... , error?: { class: TRANSIENT|PERMANENT|UNCERTAIN, code, message } }`, where `code` is one of the fourteen error codes of ADR-0079 §5. Any import outside §2–§3 makes the module fail installation.

## Host functions (module `orcon_v1`) (ADR-0032 §2)
- `http_send(req_ptr, req_len) -> i64`: request JSON `{ method, url, headers, body | bodyStream, credential?: handle, timeout }`; executed by the egress client with permission checks and credential application; returns response JSON with `bodyStream` handle for large bodies. Blocking from the guest's view; the host performs I/O on its own goroutine.
- `stream_read(h, ptr, len) -> i32`, `stream_write(h, ptr, len) -> i32`, `stream_close(h)`.
- `items_next(ptr, len) -> i32` (pull next input page), `items_emit(ptr, len) -> i32` (emit output items; counted against output caps).
- `blob_open(ref_ptr, ref_len) -> i32`, `blob_create(media_ptr, media_len) -> i32` (streams; ownership checked).
- `credential_hmac(handle, alg, data_ptr, data_len, out_ptr) -> i32` (HMAC with a credential field as key, without exposing it).
- `credential_field(handle, name_ptr, name_len, out_ptr, out_cap) -> i32` (only if `credentialRawAccess` was granted; otherwise returns a permission error).
- `log(level, ptr, len)` (redacted, capped), `now() -> i64` (recorded activation time), `random(ptr, len)` (seeded ChaCha8), `cancelled() -> i32`.
Every call is checked against the grant of the installed version; violations return an error code and are counted; three violations abort the invocation with `PermissionDenied` (`PERMANENT`).

## WASI subset (ADR-0032 §3)
Provided: `random_get` (seeded), `clock_time_get` (recorded time; monotonic clock allowed for durations), `fd_write` on fds 1/2 (to `log`), `proc_exit`, empty `args_*`/`environ_*`. Every other WASI function returns `ENOSYS`/`EBADF`. No preopened directories, no sockets.

## Limits (ADR-0032 §4)
- Linear memory maximum: manifest `resources.memoryMiB` (default 64, max 256), capped by instance policy; counted in the execution's memory accounting (ADR-0016 §6).
- Time: per-invocation deadline (default 30 s, max 300 s; `trigger.verify/handshake/decode` 50 ms per ADR-0020 §3) enforced with wazero's close-on-context-done; no instruction-count metering (documented residual risk).
- Host-call quotas per invocation: HTTP requests (default 1000), total response bytes (default 256 MiB, streamed to blobs above 64 MiB), concurrent streams (8), output items/bytes (execution limits), log bytes (1 MiB).
- Breach → that activation fails with a `PERMANENT` error naming the limit; nothing else is affected.

## Trigger families (ADR-0032 §6)
WASM connectors may implement actions and the `WEBHOOK`, `APP_EVENT` and `POLL` trigger families. `LISTEN` (persistent connections) is native-only in V1.

## ABI evolution (ADR-0032 §8)
`orcon_abi_version` selects the host implementation; the host supports every ABI version still referenced by installed packages. A binary encoding (MessagePack) is adopted in a future ABI version only if the published measurement shows host-call overhead above 20 µs per call (owner decision, SPEC-003 Q12). A future ABI may adopt the Component Model when the runtime supports it. Each new ABI version requires its own ADR.
