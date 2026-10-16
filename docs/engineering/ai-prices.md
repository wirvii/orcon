<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Maintaining the AI model prices

Status: Accepted (SPEC-020)

Orcon ships a table of model prices inside the binary. Those prices decide how much a flow is allowed to spend, so a wrong price makes a spending cap wrong. This page tells maintainers how to confirm each entry, when to do it and what to do when the price page shows something the format cannot hold. The format is in [`docs/reference/ai-prices-1.md`](../reference/ai-prices-1.md) and the decisions are in ADR-0062 and its SPEC-020 amendment.

## Rules
- The table is the file `internal/ai/cost/prices/embedded.json`. It is data. Code and tests never carry a real price.
- A person writes every price, every long-context threshold and every `verifiedAt`. Nobody fills a value in from memory, from another entry or from a tool that guesses, and nobody rounds one. A value that was not read on the page that day does not go in.
- Releases are automated by release-please (orcon-contributing RL-2). Updating prices is a normal pull request, not a step of the release.
- Test data in other changes uses obviously synthetic values in test files, never in `embedded.json`.

## Confirm an entry
Do the following for every entry you touch, one entry at a time.

1. Open the address in the entry's `source`. It must be the public page of the model provider that lists the model's prices. Write down the date and time you opened it.
2. Find the exact model name that flows use in `model.name`. Compare it character by character with the `model` field. An alias and a dated version of the same model are separate entries with the same prices; there is no prefix match.
3. Read, on the page, the price of input tokens, output tokens, cache reads, cache writes and, if the model has one, reasoning tokens. If the page lists a higher price above an input size, read that size and those prices too: they go in `longContext`.
4. Convert each price to micro-USD per million tokens: multiply the dollars per million tokens by 1000000. The result is an integer. If the conversion is not an integer, the page shows more precision than the format holds: stop (see "When the page shows something the format cannot hold").
5. Write the values into the entry. Set `verifiedAt` to the moment you read the page, as an RFC 3339 instant in UTC (`...Z`).
6. Raise `version` to `embedded-YYYY-MM-DD` and `issuedAt` to the same day, in UTC. The date in `version` must equal the date of `issuedAt`, and every `verifiedAt` must be on or before `issuedAt`.
7. Ask a second person to repeat steps 1 to 4 for every entry and write down, entry by entry, whether the page and the file match. The pull request is not approved until that comparison is written in it.

Run `make check` before you push. `TestEmbeddedTableIsConfirmed` fails when a `source` host is not on the allowed list, when a date is out of order, when an entry is duplicated, when a price is out of range or when the currency is not `USD`. It does not check how old the table is, so a green build says nothing about whether the prices are current.

## Add a model or an alias
Add a new entry with its own `source` and `verifiedAt`; do not copy the prices of another entry without reading them on the page. When a provider announces a future price change, add a second entry for the same model with the new `validFrom`: the entry with the greatest `validFrom` that is not later than now applies.

If a new provider needs a new `source` host, the host list in the cost package changes in the same pull request, and a reviewer confirms the host is the provider's own domain. A provider name may be written only where `.provenance-allow` already allows it; docs and tooling outside those paths say "the model provider".

## When to do it
- When the age notice arrives. A scheduled workflow opens or updates one issue named "Confirm AI model prices" when `issuedAt` in `main` is more than 90 days old, and lists the entries with their sources. It never fails a build.
- When a provider announces a price change.
- When a model adapter is added: the models it can call need entries before the adapter is released.
- In the user interface, people see a notice when the oldest default entry was verified more than 120 days ago, so a table left alone is visible to customers.

## When the page shows something the format cannot hold
Examples: several cache-write prices that depend on how long the cache lives, prices that change by region or by batch use, a price per request, or more precision than six decimals per million tokens. Stop. Do not choose a reading yourself and do not drop the part that does not fit. Ask the owner, who decides whether the entry is left out, the format is amended through an ADR amendment, or an approximation is accepted in writing in the pull request.

## Between releases
An operator can set prices for the instance with the file named by `ai.prices.file` or through the API, and an organization can set its own override with a note that explains why. Neither replaces confirming the embedded table: they are for a negotiated price or for the time before the next release.
