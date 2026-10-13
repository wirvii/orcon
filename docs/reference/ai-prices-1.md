<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# AI price table `orcon/ai-prices@1`

Status: **Accepted** | Source: ADR-0062 §1 and §3 (accepted 2026-10-12 via SPEC-006)

This page describes the document format of the AI price table. ADR-0062 is normative; if this page and the ADR disagree, the ADR wins. The page lists no prices: the default table is embedded in the binary and updated through signed table packages (see Distribution), so its entries are data and not part of this reference.

## Document
```
{
  version, issuedAt,
  entries: [{
    provider, model, region?,
    unit: PER_MILLION_TOKENS,
    input, output, cacheRead, cacheWrite,
    reasoning?, embedding?, perRequest?,
    validFrom
  }]
}
```
Field names are identifiers; the enumerated value of `unit` is UPPER_SNAKE (ADR-0080 §3).

## Cost calculation
Every model, embedding, rerank, judge, summary and query-rewrite call records its `usage` and a cost `cost_microdollars = Σ tokens × price`, using the price table in force. The call also records `price_table_version` and a cost status: `EXACT`, `ESTIMATED`, `POSSIBLY_BILLED` or `UNKNOWN`. `ESTIMATED` is the default, `UNKNOWN` is recorded when the model has no price, and `POSSIBLY_BILLED` is recorded for attempts lost in flight.

Provider invoices are authoritative; Orcon reports estimates.

## Distribution
- A default copy is embedded in the binary.
- Updates need no release: a catalog package of kind `PRICE_TABLE`, signed through the TUF catalog (ADR-0033), is refreshed with the catalog metadata (default every 6 hours) or imported offline with `orcon ai prices import`.
- Overrides exist at two levels, both audited: instance (permission `instance:ai-prices`) and organization (permission `ai-price:manage`).
- Resolution order: organization override, then instance override, then the newest valid signed table, then the embedded table.

## Storage
`ai_price_table(id, version, source, document, document_format, verified_at)` is an instance table. Organization overrides are stored in `ai_price_override` and instance overrides in `instance_ai_price_override`.
