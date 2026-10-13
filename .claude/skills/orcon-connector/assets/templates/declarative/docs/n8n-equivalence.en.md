<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Acme: n8n equivalence

This connector replaces the n8n node `n8n-nodes-base.acme` (type version 1). Imported flows keep working without changes, except for the differences below.

## Operations

| n8n | Orcon | Difference |
|---|---|---|
| Contact: Create | `contact.create` | none |
| Contact: Get | `contact.get` | none |
| Contact: Get All | `contact.getMany` | renamed verb; `returnAll` and `limit` keep their meaning |
| Contact: Update | `contact.update` | none |
| Contact: Delete | `contact.delete` | none |
| Message: Send | `message.send` | none |

## Parameters and defaults

- `limit` defaults to 50, as in n8n.
- `updatedAfter` is new; imported flows leave it empty.

## Repeat safety

n8n retries every operation the same way. In Orcon, `message.send` is never repeated automatically: if Acme answers with a server error after the message was sent, a person is asked to confirm whether it was delivered.

## Missing operations

None.
