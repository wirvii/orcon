<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
## API checklist

<!-- Paste this block into the description of every pull request that changes the OpenAPI contract (packages/openapi/), a CLI command, an MCP tool, a webhook payload or the contract lint. Rules: .agents/skills/orcon-api/references/. -->

- [ ] The authorizing spec is accepted (cite it: `SPEC-…`, `BL-…`, `docs/adr/…`).
- [ ] Paths follow the grammar and actions use one of the 31 listed verbs (paths.md).
- [ ] No envelope: the resource, `{ items, nextPageToken? }` or problem+json (responses.md).
- [ ] Fields, enums (UPPER_SNAKE, including values inside documents returned verbatim), ids and times follow fields.md.
- [ ] Lists take `limit` and `pageToken`, typed filters and a sort allowlist with a default (pagination.md).
- [ ] Every problem code the operation can return is listed and in the catalog (errors.md).
- [ ] Idempotency and `If-Match` are declared where required (idempotency-and-concurrency.md).
- [ ] The rate class matches the operation (rate-limits.md).
- [ ] Long work returns `202` with an `Operation` (long-running.md).
- [ ] Events are in `webhooks` with `x-orcon-event`; payloads are thin and carry no secrets (events.md).
- [ ] Stability is declared; no breaking change on stable or beta (`make api-compat` output below) (versioning.md).
- [ ] Deprecations carry `x-orcon-sunset` and `x-orcon-deprecation-guide`.
- [ ] Every operation declares a CLI command or an exemption with a reason (channels.md).
- [ ] The MCP level is correct; a new tool has an accepted spec.
- [ ] Permission and audit action are declared on every operation.
- [ ] Summary, description and field documentation are complete (documentation.md).
- [ ] Every example has a registered scenario and ran green on SQLite (`make api-examples`); the last nightly PostgreSQL run is green.
- [ ] The reference site builds (`make api-reference`).
- [ ] `make generate` output is committed.
- [ ] No secrets or personal data in examples.

### Commands run

<!-- List each command and its result, for example: make api-check (pass), make api-examples (pass). -->

-

### make api-compat output

```text
<!-- paste the output of make api-compat here -->
```
