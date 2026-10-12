<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# API and code generation

Status: Proposed — pending owner approval

This document defines how Orcon's HTTP API is specified, generated and served. It applies to `packages/openapi/v1/`, `internal/adapter/httpapi` and its generated package `apigen`. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`API-…`) are stable. The foundational choices are recorded in ADR 0076 §1–§2.

## Spec-first

- **API-1** The OpenAPI 3.1 document in `packages/openapi/v1/` is the source of truth for the public HTTP API. Change the document first, then regenerate, then implement.
- **API-1.1** Handlers implement `apigen.StrictServerInterface`; no route is registered by hand for an operation that the document describes.
- **API-1.2** OpenAPI 3.1 features that the generator does not support are not used in the document (ADR 0054); if an essential feature is missing, the owner decides.

## Generation

- **API-2** `make generate` runs oapi-codegen v2 with `internal/adapter/httpapi/apigen/oapi-codegen.yaml`: `std-http-server`, `strict-server`, models and the embedded spec, nullable types enabled, output `server.gen.go`. The input is the bundled document `packages/openapi/v1/openapi.bundle.yaml`.
- **API-2.1** Generated code is committed. `make generate-check` regenerates and fails when the result differs from the committed files.
- **API-2.2** Generated files are never edited by hand; they carry the generator's `DO NOT EDIT` header and are excluded from lint by `exclusions.generated: strict`.
- **API-2.3** oapi-codegen is pinned as a `tool` directive in `tools/go.mod`; its runtime package `github.com/oapi-codegen/runtime` is imported only by `internal/adapter/httpapi` (architecture-layers.md AL-2.3).

## Handlers

- **API-3** Handlers translate only: request types to area inputs, area results to response types, area errors to problem responses. They contain no business rules and no SQL.
- **API-3.1** Handlers take the request context and pass it to the area unchanged (go-style.md GS-C5).
- **API-5** Middleware has the signature `func(http.Handler) http.Handler`. Instrumentation uses `otelhttp`. Routing uses the standard library `http.ServeMux` with Go 1.22+ patterns; Fiber and fasthttp are banned.
- **API-5.1** Every `http.Server` sets `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` and `IdleTimeout`; long-lived streams (SSE) extend their write deadline per write with `http.ResponseController`.
- **API-5.2** Request bodies are bounded with `http.MaxBytesReader` at a documented limit per operation.

## Errors at the edge

- **API-4** One error-mapping table in `internal/adapter/httpapi` maps area sentinels and typed errors to `application/problem+json` responses (status, type URI, title) as defined by SPEC-005. It is the only place that knows HTTP status codes (go-style.md GS-E3).
- **API-4.1** An error not in the table becomes `500` with a correlation ID in the problem body; the error is logged once with the same ID (GS-E4). Internal error text never reaches the client.
- **API-4.2** Validation errors return every invalid field in one response, not only the first.

## Same contract everywhere

- **API-6** The same OpenAPI document drives the generated server, the client SDKs, the CLI command metadata and the MCP tool catalog. A capability is added to the document once and appears consistently in each surface.
- **API-6.1** A breaking change to the document requires a new API version path (`/v2`) and is approved like any foundational change.
