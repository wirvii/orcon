<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Naming

Status: Accepted

This file fixes the names of connectors, nodes, resources, operations and parameters, the date format, the catalogs and the spelling of enumerated values. Rule IDs (`NM-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **NM-1** Connector id: `orcon.<service>` for official, `<publisher>.<service>` otherwise; lowercase kebab-case segments; the service's own name, never a trademark of another product; never `n8n`.
- **NM-2** Nodes: `name` in lowerCamelCase; one action node per connector (`<service>`), one trigger node (`<service>Trigger`) when needed; node type `<id>.<node>@<major>`.
- **NM-3** Resources: singular lowerCamelCase nouns (`contact`, `dealStage`, `file`). Uncountable or invariable nouns are allowed (`status`, `news`); anything else plural needs `authoring.json` `resourceExceptions[] { resource, reason }`.
- **NM-4** Operations are `<resource>.<verb>` with a verb from this closed vocabulary (17):

  | Verb | Meaning |
  |---|---|
  | `create` | create a new record |
  | `get` | read one record by id |
  | `getMany` | list records (NM-8) |
  | `update` | change fields of an existing record |
  | `upsert` | create or update by a matching key |
  | `delete` | delete a record |
  | `send` | transmit a message or notification |
  | `search` | query by text or filter with provider ranking or syntax |
  | `add` | attach an existing item to a container (member to list, label to item) |
  | `remove` | detach an item from a container without deleting it |
  | `archive` | move a record to an archived state |
  | `unarchive` | restore an archived record |
  | `upload` | transfer a binary file to the service |
  | `download` | transfer a binary file from the service |
  | `copy` | duplicate a record or file |
  | `move` | change the location or parent of a record or file |
  | `cancel` | stop a pending or running process on the service (order, job, subscription) |

- **NM-5** Any other verb needs `authoring.json` `verbExceptions[] { operation, reason }`, where the reason says why no vocabulary verb fits. Reviewers reject exceptions that a vocabulary verb covers.
- **NM-6** Parameters: names in lowerCamelCase; types only from ADR-0030 §3 (`STRING`, `NUMBER`, `BOOLEAN`, `OPTIONS`, `MULTI_OPTIONS`, `COLLECTION`, `FIXED_COLLECTION`, `JSON`, `DATE_TIME`, `RESOURCE_LOCATOR`, `BINARY_PROPERTY`); booleans named as positive questions (`includeArchived`); option values a connector defines for its own parameters are lowerCamelCase (connector data, NM-13).
- **NM-7** The identifier of the target record is the parameter `<resource>Id` of type `RESOURCE_LOCATOR` (modes `LIST`, `ID`, `URL` when the service has URLs).
- **NM-8** Every `getMany` has `returnAll` (`BOOLEAN`, default false) and `limit` (`NUMBER`, default 50, minimum 1, shown when `returnAll` is false). With pagination, `returnAll` follows pages up to `maxPages`; without pagination, `limit` is applied by the connector.
- **NM-9** Filters of `getMany` and `search` live in a `filters` collection; sort in `sort`.
- **NM-10** Output items keep the provider's record unchanged and always have a top-level `id` string; when the provider names it differently the response map adds `id` and keeps the original field.
- **NM-11** Dates the connector defines (parameters of type `DATE_TIME`, fields it adds, metadata) are RFC 3339 in UTC. Provider fields in outputs are returned as received.
- **NM-12** Display names, descriptions, option labels, placeholders and hints are written in English in the manifest and listed in `i18n/en.json` (identical) and `i18n/es.json` under stable keys (ADR-0079 §2). Wording follows `../../orcon-ui/references/content-and-i18n.md`.
- **NM-13** Value casing (ADR-0079 §14, ADR-0029 §10). Every enumerated or state value written in the manifest, the declarative body, `authoring.json` and `tests/live.json` is UPPER_SNAKE (`EFFECT`, `UNSAFE`, `NATIVE`, `UNCERTAIN`, `DATE_TIME`). Not affected: field names (lowerCamelCase); identifiers — connector ids, node types and names, resources, operations and verbs, error codes, credential type ids, change-descriptor operations; values fixed by external standards — HTTP methods, URL schemes (`https`), algorithm names; option values a connector defines for its own parameters; and recorded tests in `orcon/cassette@1`, which keep the recorded values as they were. The manifest schema rejects any other spelling, so a lowercase value fails `lint` before review.
