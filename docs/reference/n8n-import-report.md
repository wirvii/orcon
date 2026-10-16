<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# n8n import report

Status: Proposed — pending owner approval | Source: SPEC-021, ADR-0084 (Proposed)

The import report is the document that says what happened to each part of an n8n workflow when Orcon converted it. Its format is `orcon/n8n-import-report@1`. It is the same in the editor, in the API (`evaluateN8nConverter` and `Import.workflows[].report`), in the output of `orcon n8n convert --report-json` and in the stored import. Messages are shown in the language of the request (English or Spanish); the stored report keeps only codes and parameters.

The report never contains a parameter value, an expression or a secret. It names node names, node types, field pointers (RFC 6901 JSON Pointers inside the n8n node), counters and credential names.

Every code below is stable. New codes are added without changing the meaning of existing ones. A code that appears in the converter and is missing from this page is a defect.

## Node outcomes

Each node of the workflow gets exactly one outcome.

| Code | Meaning | What to do |
|---|---|---|
| `CONVERTED` | The node runs the same way as in n8n. | Nothing. |
| `ADAPTED` | The node runs, with a difference the report states. | Read the difference and confirm it is acceptable. |
| `NEEDS_REVIEW` | A person has to look at the node: it is marked unsafe, it uses a credential that has no secret yet, or one of its expressions has findings. | Open the node, review it and fix what the findings list. |
| `UNSUPPORTED` | The node has no Orcon equivalent. It becomes a placeholder (`orcon.unsupported@1`), always disabled, that keeps the original node with its connections and position. | Rebuild the node with a node Orcon has, usually from the suggestion. |

## Reasons a workflow is skipped

A workflow that cannot be converted is `SKIPPED`; the other workflows of the same import are not affected.

| Code | Meaning | What to do |
|---|---|---|
| `NOT_A_WORKFLOW` | The item has no `nodes` list, or a node has no `type` or `typeVersion`. The report gives the position of the item in its source. | Export the workflow again from n8n. |
| `TOO_MANY_NODES` | The workflow has more than 2,000 nodes or more than 10,000 connections. | Split the workflow in n8n and import the parts. |
| `DOCUMENT_TOO_LARGE` | The converted document is larger than 8 MiB, the size limit of a draft. | Split the workflow. |
| `DUPLICATE_NODE_NAME` | Two nodes of the workflow have the same name. The report gives the name. | Rename one of the nodes in n8n and export again. |
| `CONVERSION_TIMEOUT` | The conversion of the workflow used its 10 second budget. | Split the workflow; if it persists, report it. |
| `CONVERTER_DEFECT` | The converted document failed Orcon's structural validation. This is a defect of the converter, not of your workflow. The report gives the request id. | Report it with the request id. |

## Findings

A finding is a note about one part of a node. It carries a field pointer when it concerns one field, and may carry a suggestion.

| Code | Meaning | What to do |
|---|---|---|
| `UNMAPPED_TYPE` | The converter has no mapping for this n8n node type. | Follow the suggestion, or rebuild the node. |
| `UNMAPPED_TYPE_VERSION` | The type is mapped, but not this version of the node. | Rebuild the node, or export it from n8n with a version Orcon maps. |
| `UNMAPPED_OPERATION` | The resource and operation pair of the node has no mapping. | Rebuild the operation, for example with the HTTP node. |
| `SELECTOR_EXPRESSION` | The resource or the operation of the node is an expression, so the converter cannot tell which operation it runs. | Choose the operation by hand. |
| `UNMAPPED_CREDENTIAL` | No Orcon credential type is declared for the n8n credential type. | Create a credential of a type Orcon has and select it on the node. |
| `PLANNED_ONLY` | The connector for the node exists in the roadmap but is not installed. | Wait for the connector, or use the HTTP node. |
| `UNSUPPORTED_CORE` | A core node that has no Orcon counterpart yet. | Rebuild it with other nodes. |
| `NOT_AVAILABLE` | The Orcon counterpart of the node is not available in this release. It replaces the code `NOT_ON_MAIN` of earlier converter versions. | Wait for a release that has it, or rebuild the node. |
| `REVIEW_REQUIRED` | A person must review the node before the flow can be activated. | Review the node and mark it reviewed. |
| `UNSUPPORTED_PARAMETER` | A parameter of the node has no Orcon counterpart and was not converted. | Set the behavior by hand if you still need it. |
| `UNSUPPORTED_CONNECTION` | A connection of the node has no Orcon counterpart. | Rebuild that part of the flow. |
| `DANGLING_CONNECTION` | A connection names a node the workflow does not have. | Check the flow after import. |
| `DISABLED` | The node was switched off in n8n; the Orcon node stays disabled. | Enable it if you want it to run. |
| `ADAPTED` | The node was converted with a difference the report shows, such as a sticky note cut to fit the editor. | Read the difference. |
| `SECRET_VALUE` | A field looked like a secret. The value was replaced by `NEEDS_SECRET` and the node became a placeholder. The report names the field, never the value. | Enter the secret in a credential. |
| `NEEDS_SECRET` | The node uses a secret that is not imported. | Write the secret into the credential of the node. |
| `NO_RUNNABLE_TRIGGER` | The workflow has no trigger that can start it: the original trigger became a placeholder. | Add a trigger Orcon has. |
| `FLOW_RENAMED` | The name of the workflow was already used by another flow of the project, so a number was added: `(2)`, `(3)`. | Rename the flow if you want another name. |
| `NODE_RENAMED` | A node name was already used in the draft the nodes were appended to, so a number was added: `2`, `3`. | Nothing; references were updated. |
| `REFERENCE_REWRITTEN` | An expression referred to a renamed node by name and was rewritten to the new name. | Check the expression. |

### Expression findings

Every string that starts with `=` is copied unchanged and then analyzed. Nothing is evaluated. A node with any of these findings has the outcome `NEEDS_REVIEW`; they do not block activation, and the editor underlines the expression.

| Code | Meaning | What to do |
|---|---|---|
| `EXPRESSION_SYNTAX` | The expression has a syntax error. | Fix the expression in the editor. |
| `EXPRESSION_UNKNOWN_IDENTIFIER` | The expression uses a `$` name Orcon does not have. | Use the Orcon equivalent. |
| `EXPRESSION_UNKNOWN_METHOD` | The expression calls a method Orcon does not have. | Use another method. |
| `EXPRESSION_DYNAMIC_REFERENCE` | The expression refers to a node by a computed name, so it cannot be checked or rewritten. | Replace it with a literal node name. |
| `EXPRESSION_REFERENCES_UNSUPPORTED_NODE` | The expression refers to a node that became a placeholder. | Rebuild that node first. |
| `EXPRESSION_ENV_DISABLED` | The expression reads `$env` and environment access is disabled. | Use a variable instead, or ask an administrator to enable `$env`. |
| `EXPRESSION_SECRETS_UNRESOLVED` | The expression reads `$secrets.<provider>.<name>` and no secret store with that alias exists. | Create the secret store with that alias. |

## Differences

A difference is a change of behavior between n8n and Orcon for the whole workflow. The set grows by adding codes.

| Code | Meaning | What to do |
|---|---|---|
| `WAS_ACTIVE_IN_N8N` | The workflow was active in n8n. In Orcon it is inactive until you activate it. | Activate it after the review. |
| `SEQUENTIAL_BRANCHES` | Branches run one after the other, in the order n8n used. | Nothing. |
| `LEGACY_EXECUTION_ORDER` | The workflow used the legacy execution order (`v0`). Orcon does not reproduce it. | Check the order in which branches run. |
| `TIME_CONSTANT_PER_STEP` | An expression uses `$now`, `$today` or `Date.now()`. They return the time recorded when the step started. | Check expressions that compare times inside one step. |
| `WEBHOOK_WAIT_LIMIT` | A webhook answers with the last node or a response node. The wait has a time limit of 300 seconds. | Check that the caller can wait that long. |
| `STATIC_DATA_NOT_CARRIED` | The workflow has static data. It is not carried over. | Move what you need into variables or a store. |
| `TAGS_NOT_CARRIED` | The workflow has tags. They are not carried over. | Tag the flow in Orcon. |
| `PIN_DATA_NOT_IMPORTED` | The workflow has pinned data on some nodes (the report gives the number). It is not imported. | Pin the data again in the editor. |
| `ERROR_WORKFLOW_UNRESOLVED` | The workflow names an error workflow that is not in the same import. | Select the error flow in the settings of the flow. |
| `SETTINGS_NOT_CARRIED` | Some workflow settings are not carried over. The report lists their keys. | Set what you need in Orcon. |
| `ADDRESS_IN_USE` | The path of a webhook is already used by another flow of the project. Activation will fail. | Change one of the paths. |

## Suggestions

A suggestion accompanies a finding and says what to do next.

| Code | Meaning | What to do |
|---|---|---|
| `USE_HTTP_NODE` | The generic HTTP node can do the job. | Build the call with the HTTP node. |
| `INSTALL_CONNECTOR` | A connector for this service exists but is not installed in the project. | Install the connector, then import again. |
| `CONNECTOR_PLANNED` | A connector for this service is planned. | Use the HTTP node meanwhile. |
| `REBUILD_NODE` | The node has to be rebuilt with Orcon nodes. | Rebuild it. |
| `CREATE_CREDENTIAL` | A credential has to be created. | Create it and write its secret. |
| `SET_ALLOWED_HOSTS` | The credential is used for generic HTTP authentication. | Set the hosts the credential may be sent to. |

## Credential resolutions

Each credential an n8n node refers to gets one resolution, grouped by a stable `key` (the first 16 hexadecimal characters of the SHA-256 of the n8n type, the id and the name, separated by a zero byte).

| Code | Meaning | What to do |
|---|---|---|
| `EXISTING` | A credential of the project with the same name and the same type was linked, or you chose one. | Nothing. |
| `PLACEHOLDER_CREATED` | A credential with the same name and the Orcon type was created without a secret. Its status is `NEEDS_SECRET`. | Write the secret. |
| `PLACEHOLDER_PENDING` | You lack the permission to create credentials. The nodes keep the name, and the report says who has to create it. | Ask someone who can create credentials. |
| `UNMAPPED` | No Orcon credential type matches. The node that uses it becomes a placeholder. | See `UNMAPPED_CREDENTIAL`. |

## Gates

A gate says what is blocked until you act. The report lists the nodes or credentials that cause it.

| Code | Meaning | What to do |
|---|---|---|
| `PLACEHOLDER_NODE` | The flow has a placeholder node. It cannot be published. | Rebuild the node or delete it. |
| `CREDENTIAL_NEEDS_SECRET` | The flow uses a credential that has no secret. It cannot be published. | Write the secret. |
| `REVIEW_PENDING` | A node is marked for review. The flow cannot be activated. | Review each marked node. |

Each node also lists what it blocks: `PUBLICATION` for a placeholder or a node that uses a credential without a secret, and `ACTIVATION` for a node marked for review.

## Review reasons

A node marked for review carries one of these reasons (ADR-0034 §8).

| Code | Meaning | What to do |
|---|---|---|
| `HTTP_WRITE` | An HTTP request that changes data and cannot be shown safe to repeat. | Review it before you activate the flow. |
| `HTTP_METHOD_EXPRESSION` | The HTTP method is an expression, so its safety cannot be known. | Use a fixed method, so the node can be checked. |
| `UNSAFE_OPERATION` | The operation has no safety contract Orcon can infer. | Review it. |

## Limits of the report

- At most 20 findings per node; `truncatedFindings` says how many were left out.
- At most 2 MiB per workflow.
