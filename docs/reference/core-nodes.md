<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Core data nodes

Status: **Proposed** | Source: SPEC-003 Annex B (cut CN13), ADR-0007, ADR-0008, ADR-0018

The core data nodes are pure native nodes that no connector package owns. Annex B fixes their type names (`orcon.<name>@1`), their purity (`PURE`) and the n8n nodes they replace. It does not fix their parameters: the schemas below are the ones the implementation proposes, derived from the public description of the n8n nodes, and they need the architect's ratification before they are Accepted.

Rules that every node here follows:

- The parameters object is closed. An unknown key is an error, so a misspelled parameter never silently does nothing.
- Enumerated values are UPPER_SNAKE (ADR-0080 section 3).
- A parameter is a literal unless it is documented as an expression. A string that starts with `=` in a literal parameter is refused; the evaluated parameters are the Set values (`orcon.set@1`) and the Filter condition.
- Expressions see only the current item (`$json`, and `$input` holding that one item) and the recorded time and seed of the activation. Reading other nodes with `$('Name')` is not available to a core node yet (gap G2).
- Every node has one output port, `main`. An item that leaves a node keeps the lineage of the input item it comes from (ADR-0008 section 1).
- Invalid parameters fail the node with the permanent error `INVALID_PARAMETERS`; a failure while processing items fails it with the permanent code named below.
- A field name is a key of the item. When the item has no key with that exact name, a name with dots is read as a path through nested objects (`customer.address.city`); arrays are not entered.

## `orcon.filter@1` (n8n Filter)

Keeps the items a condition accepts, in their order.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `condition` | string | yes | The test each item must pass. Plain text is the code of one expression segment; a string that starts with `=` is a template (ADR-0018 section 1). It must evaluate to a boolean. |

Failure code: `FILTER_FAILED` (the condition is not a boolean, or it fails to evaluate). The kept items are the input items themselves.

## `orcon.sort@1` (n8n Sort)

Orders the items. The sort is stable.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `mode` | `SIMPLE` or `RANDOM` | no, default `SIMPLE` | `SIMPLE` orders by fields; `RANDOM` shuffles with the seed recorded for the activation, so a recomputed activation gives the same order. |
| `fields` | list of `{field, order}` | in `SIMPLE` mode, at least one | `field` is a field name; `order` is `ASCENDING` (default) or `DESCENDING`. A field is listed once. Empty in `RANDOM` mode. |

Order between values: a missing field or null first, then booleans (`false` before `true`), numbers by value, strings by UTF-16 code unit, arrays and objects by their canonical JSON text. `DESCENDING` reverses all of it. The n8n code mode (sorting with JavaScript) is not part of this node.

## `orcon.limit@1` (n8n Limit)

Keeps at most a number of items.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `maxItems` | whole number, 1 to 1000000000 | yes | How many items stay. |
| `keep` | `FIRST_ITEMS` or `LAST_ITEMS` | no, default `FIRST_ITEMS` | Which end of the input stays. |

## `orcon.removeDuplicates@1` (n8n Remove Duplicates)

Drops the items that repeat an earlier item of the same input. The first item of each group stays, in input order. Two items are the same when their compared values are the same JSON value: the order of the members of an object does not matter, `1`, `1.0` and `1e0` are one number, and no number is rounded, so two long integers a double cannot tell apart stay different.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `compare` | `ALL_FIELDS`, `ALL_FIELDS_EXCEPT` or `SELECTED_FIELDS` | no, default `ALL_FIELDS` | What makes two items the same. |
| `fields` | list of field names | for the last two | The fields to ignore (`ALL_FIELDS_EXCEPT`) or to compare (`SELECTED_FIELDS`). Empty for `ALL_FIELDS`. |
| `removeOtherFields` | boolean | no, default `false` | With `SELECTED_FIELDS` only: the items that stay hold only the compared fields, each under its full name. |

Failure code: `REMOVE_DUPLICATES_FAILED` (an item is not a JSON object). Remembering items across executions (n8n's other operations) needs flow data and is not part of this node.

## `orcon.aggregate@1` (n8n Aggregate)

Gathers the input into one item. With no input items it emits no item. The item carries the lineage of every input item.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `mode` | `INDIVIDUAL_FIELDS` or `ALL_ITEM_DATA` | no, default `INDIVIDUAL_FIELDS` | What is gathered. |
| `fields` | list of `{field, outputField}` | `INDIVIDUAL_FIELDS`, at least one | The values of each `field`, in item order, become a list under `outputField` (the field name when absent). Output names are unique. |
| `mergeLists` | boolean | `INDIVIDUAL_FIELDS`, default `false` | A value that is a list adds its elements instead of itself. |
| `keepMissing` | boolean | `INDIVIDUAL_FIELDS`, default `false` | An item without the field adds `null`; otherwise it adds nothing. |
| `destinationField` | field name | `ALL_ITEM_DATA`, default `data` | The field of the output item that holds the list of items. |
| `include` | `ALL_FIELDS`, `SELECTED_FIELDS` or `ALL_FIELDS_EXCEPT` | `ALL_ITEM_DATA`, default `ALL_FIELDS` | Which fields of each item go into the list. |
| `includeFields` | list of field names | `ALL_ITEM_DATA`, for the last two | The fields to keep or to leave out. |

A parameter of the other mode is an error. Failure code: `AGGREGATE_FAILED` (an item is not a JSON object, in `ALL_ITEM_DATA`).

## `orcon.splitOut@1` (n8n Split Out)

Makes one item for each element of a list field.

| Parameter | Type | Required | Meaning |
|---|---|---|---|
| `field` | field name | yes | The list to split. |
| `destinationField` | field name | no, default the name of `field` | The field of each new item that holds one element. The element is written there as it is, an object included. |
| `include` | `NO_OTHER_FIELDS`, `ALL_OTHER_FIELDS` or `SELECTED_OTHER_FIELDS` | no, default `NO_OTHER_FIELDS` | Which other fields of the input item the new items keep. `ALL_OTHER_FIELDS` leaves out the split field. |
| `includeFields` | list of field names | with `SELECTED_OTHER_FIELDS` | The fields to keep. |

An empty list gives no item. An item whose field is missing, null or not a list fails the node with `SPLIT_OUT_FAILED`; each new item carries the lineage of the item it comes from.

## Open design points

| Id | Point | What the implementation does meanwhile |
|---|---|---|
| G1 | Annex B names the types but no parameter schema for the CN13 nodes. | The schemas above, for ratification. |
| G2 | A core node cannot read the output of another node (`$('Name')`, `$input.all()`), because the node call carries only the input batch. | Expressions see the current item only. |
| G3 | Expressions in the literal parameters (for example a `maxItems` computed per run) have no defined evaluation point. | They are refused; the n8n converter reports them as unsupported. |
| G4 | The editor draws the icon of a built-in node from its own table; the node type API carries icons only for connector brands. | No icon is sent by the API for these nodes; the editor table needs an entry per node. |
| G5 | n8n's Item Lists (legacy) is one node with several operations; Annex B maps it to `orcon.<name>@1` without saying which node each operation becomes. | Not converted yet; the importer reports it as not on main. |
| G6 | Annex B does not say what Orcon does with an item that is not a JSON object in these nodes, nor with several fields in one Split Out. | The node fails the item list for the first (permanent error); the converter reports the second as unsupported. |
