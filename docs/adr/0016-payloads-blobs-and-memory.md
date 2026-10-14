<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0016 — Large payloads, blobs, streaming and memory caps

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
D18: inline threshold ~256 KB, blobs by reference, streaming, paging, per-execution memory cap (default 512 MB), blob deletion follows retention.

## Decision
1. Output documents ≤ 256 KiB (after compression) inline; larger → blob reference. Binary item data is always a blob.
2. `BlobStore { Put(ctx, owner, io.Reader) (BlobRef, error); Get(ctx, owner, ref) (io.ReadCloser, error); Delete }`. Backends: filesystem (standalone default, under the data directory), PostgreSQL (distributed default, 1 MiB chunks in `blob_chunk`), S3-compatible (optional; dependency licence Apache-2.0). Blob paths and object keys start with `org/<orgId>/` (SPEC-004 R-ISO-08).
3. `BlobRef = { id, size, sha256, mediaType, backend, owner(executionId | flowId) }`. Reads verify ownership (execution isolation).
4. A blob is written and synced before the checkpoint that references it commits (`committed_at` set in that transaction); uncommitted blobs older than 1 h are garbage collected.
5. Streaming: nodes receive `io.Reader` for binary data and must not buffer beyond a declared bound. Huge lists are stored as paged NDJSON blobs (1000 items/page) and iterated page by page; per-item nodes stream pages through without materializing the whole list.
6. Memory accounting per execution per worker: decoded batches held, expression/Code sandbox memory, node-declared buffers. Default cap 512 MiB (flow setting, instance maximum). Breach → that execution FAILED with `MemoryLimitExceeded`; nothing else is affected.
7. Worker budget = `min(configured, 0.8 × GOMEMLIMIT)`; admission control stops claiming at 85 %. Honest limit: Go cannot partition its heap per execution; accounting is conservative and approximate, enforced at allocation points the engine controls, with sandbox memory hard-capped by WASM.
8. Blob deletion follows ADR-0017 retention.

## Consequences
Bounded memory regardless of payload size; one runaway execution cannot exhaust a worker.

## Alternatives considered
PostgreSQL large objects (rejected: operational friction, vacuum issues); copying payloads through every token (rejected: memory and write amplification).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** Blob paths carry the prefix `org/<orgId>/`; the owner check includes the organization.
