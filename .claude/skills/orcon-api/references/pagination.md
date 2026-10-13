<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Pagination, filters and sort

Status: Accepted

This document defines how collections and views are paged, filtered and sorted. It refines the pagination convention of ADR-0053. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`PAGE-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Cursor pagination

- **PAGE-1** Every `GET` on a collection or view takes `limit` (1–200, default 50) and `pageToken`; never `offset`, `page` or `pageSize`. The response is `{ items, nextPageToken? }` ([RESP-2](responses.md)). [`list-pagination`]
- **PAGE-2** Page tokens are opaque, bind organization, filters and sort; reuse with other parameters → `invalid-page-token`. [`problem-codes-declared`]
- **PAGE-3** Ordering is total (sort key plus id). [review]
- **PAGE-4** `includeTotal=true` only where `x-orcon-include-total: true`; above 10,000 the response sets `totalIsLowerBound: true`. [`list-pagination`]

## Sort and filters

- **PAGE-5** `sort=field,-field2` from an `enum` allowlist; `x-orcon-sort-default` names the default. [`sort-allowlist`]
- **PAGE-6** Filters are typed parameters (`status`, `<resource>Id`, `createdAfter`, `createdBefore`, `tag`, `q` for name prefix); no generic filter language in v1. [review]
