<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Pagination and rate limits

Status: Accepted

This file covers how a connector pages through lists and declares the provider's rate limits (ADR-0030 §6, ADR-0014). Rule IDs (`PG-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **PG-1** Pagination only on operations whose purity is `READ`, with the strategy the provider documents (ADR-0030 §6); `maxPages` explicit; `onMaxPages: "TRUNCATE"` only when the documentation says so and the page warns.
- **PG-2** Page size is the provider maximum up to 200 unless documentation advises less.
- **PG-3** Pages stream (`emit: "STREAM"`); never accumulate all pages in memory.
- **PG-4** `rateLimits[]` are taken from the provider's published limits, with the documentation URL in `authoring.json` `rateLimitSources[]`; `key` is `CREDENTIAL` unless limits are per host (`HOST`) or per account (an expression).
- **PG-5** `respectHeaders` is set whenever the provider sends `Retry-After`, reset or remaining headers.
- **PG-6** Never add sleeps or loops in templates or native code to work around limits; the host buckets (ADR-0014) do it.
