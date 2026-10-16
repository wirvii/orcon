<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# AI price table `orcon/ai-prices@1`

Status: **Accepted** | Source: ADR-0062 §1 and §3 (accepted 2026-10-12 via SPEC-006), as amended by SPEC-020 (2026-10-16)

This page describes the document format of the AI price table. ADR-0062 is normative; if this page and the ADR disagree, the ADR wins. The page lists no prices: the default table is embedded in the binary, so its entries are data and not part of this reference. Updating the table between releases is described under Distribution.

## Document
```
{
  format: "orcon/ai-prices@1",
  version, issuedAt,
  currency: "USD",
  entries: [{
    provider, model, region?,
    unit: PER_MILLION_TOKENS,
    input, output, cacheRead, cacheWrite,
    reasoning?, embedding?, perRequest?,
    validFrom,
    source, verifiedAt,
    longContext?: { thresholdInputTokens, input, output, cacheRead, cacheWrite }
  }]
}
```
Field names are identifiers; the enumerated value of `unit` is UPPER_SNAKE (ADR-0080 §3).

### Fields added by SPEC-020
The amendment is additive: a document that does not carry the new fields stays valid.

| Field | Meaning |
|---|---|
| `format` | The literal `orcon/ai-prices@1`. It names the document type. |
| `currency` | `USD`. It is the only currency in this version. |
| `source` | The `https://` address of the public page of the provider where the price was read. Required in the embedded table; optional in an operator file. |
| `verifiedAt` | The instant, in RFC 3339 form, at which a person compared the entry with `source`. Required in the embedded table; optional in an operator file. It must not be later than `issuedAt`. |
| `longContext` | Optional. Prices for calls whose counted input is above `thresholdInputTokens`. When the counted input of a call is above the threshold, the whole call uses these prices, both when the cost is reserved and when it is recorded. The four prices follow the units of the entry. |

Amounts are integers in micro-USD per million tokens (`1 USD = 1000000`). There is no floating point in the document, in Go or in SQL.

`region`, `embedding` and `perRequest` are accepted by the parser and ignored in this version: no price is derived from them.

`version` of the embedded table is `embedded-YYYY-MM-DD` and the date equals the date of `issuedAt`. For an entry, the one with the greatest `validFrom` that is not later than the current time applies; if none applies yet, the model has no price in that layer. Model names are matched exactly: an alias and a dated version are separate entries, and there is no prefix match.

## Cost calculation
Every model, embedding, rerank, judge, summary and query-rewrite call records its `usage` and a cost `cost_microdollars = Σ tokens × price`, using the price table in force. The call also records `price_table_version` and a cost status: `EXACT`, `ESTIMATED`, `POSSIBLY_BILLED` or `UNKNOWN`. `ESTIMATED` is the default, `UNKNOWN` is recorded when the model has no price, and `POSSIBLY_BILLED` is recorded for attempts lost in flight.

Provider invoices are authoritative; Orcon reports estimates.

## Distribution
- A default copy is embedded in the binary. Each entry carries the source it was read from and the date a person confirmed it. The maintainer procedure is in `docs/engineering/ai-prices.md`.
- Between releases, an operator uses the instance layer and an organization uses its own override (see Overrides).
- A signed table of kind `PRICE_TABLE`, delivered through the TUF catalog (ADR-0033), is the third layer. In this version that layer is empty: signed tables, `orcon ai prices import` and the periodic refresh arrive with the catalog client. The refresh runs only when an administrator has explicitly installed a `PRICE_TABLE` from a remote catalog (ADR-0083 §6).
- Resolution order: organization override, then instance override, then the newest valid signed table, then the embedded table. An override replaces the whole entry of a model, never single fields.
- A failed read of the override store is an error, never a silent fall back to a lower layer, because that could bill at another price.

## Overrides
- Instance layer, two exclusive origins. When the key `ai.prices.file` (`ORCON_AI_PRICES_FILE`) is set, the file is the instance layer: it is read at startup and on each configuration reload, and writes to the instance overrides through the API are refused with `INSTANCE_AI_PRICES_MANAGED_BY_CONFIG`. An invalid file stops the startup and is reported by `orcon config check`; an invalid file in a reload is rejected and the previous layer stays. Without the key, the instance layer is the table `instance_ai_price_override`, managed through the API and the command line.
- Organization layer: the table `ai_price_override`. The key `ai.prices.orgOverrides` is `ALLOWED` (default) or `DISABLED`. With `DISABLED`, existing overrides are ignored when resolving (they are not deleted) and writes are refused with `AI_PRICE_OVERRIDES_DISABLED`.
- Each override carries a mandatory note of 1 to 500 characters that gives the reason.
- Overrides are read at most every 30 seconds per organization; a write in the same process takes effect at once. Another process can use the previous price for up to 30 seconds.

## Price version recorded on each call
`ai_model_call.price_table_version` and the version of each cost entry identify the layer and the document used:

| Layer | Version |
|---|---|
| Embedded | `embedded-YYYY-MM-DD` |
| Instance file | `config:` followed by the first 12 hexadecimal characters of the SHA-256 of the document |
| Instance override | `instance-override:<id>:<revision>` |
| Organization override | `org-override:<id>:<revision>` |

## Codes
- `AI_PRICE_UNKNOWN`: error of the agent node, permanent and routable by the node's error rules. It is raised before the model is called when the node sets `maxSpend` (or a covering budget has `unknownPrice: BLOCK`) and the model has no price. It replaces `AI_BUDGET_EXCEEDED` for this case.
- `MODEL_PRICE`: issue code of flow publication. An agent node with `maxSpend` whose model has no price is a publication error; an AI node without `maxSpend` whose model has no price is a warning. The check does not run when no price source is configured.
- `AI_PRICE_OVERRIDE_EXISTS`, `AI_PRICE_OVERRIDES_DISABLED` and `INSTANCE_AI_PRICES_MANAGED_BY_CONFIG`: conflicts (409) of the override endpoints. The catalog of problem codes is generated from the API contract in `docs/reference/problem-codes.md`.

## Storage
`ai_price_table(id, version, source, document, document_format, verified_at)` is an instance table; this version does not create it, because signed tables are deferred. Organization overrides are stored in `ai_price_override` and instance overrides in `instance_ai_price_override`.
