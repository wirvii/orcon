<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR 0084: n8n workflow import operation, report and run semantics of imported nodes

Status: Proposed

## Context
ADR-0037 and ADR-0051 define how n8n nodes and credentials map to Orcon, ADR-0052 names the
`projects/{projectId}/imports` resource and ADR-0053 §7 lists imports among long-running operations.
The converter exists, but no operation lets a person import an n8n workflow file, the report has no
public format, nothing says what a disabled node does at run time, and the engine refuses every flow
that contains one, so an imported flow with a placeholder cannot even be tested.

## Decision
1. Import of n8n workflow files supplied in the request body is synchronous and bounded:
   `POST /projects/{projectId}/imports` converts every workflow before opening a transaction and then
   writes flows, drafts, placeholder credentials, the import record, audit entries and the idempotency
   record in one transaction; `201` with the `Import` resource. Limits: 8 MiB body, 20 sources,
   50 workflows, 2,000 nodes and 10,000 edges per workflow, 10 s conversion budget per workflow, two
   concurrent conversions per organization. Imports that read a remote system stay long-running.
2. Targets: new flows, or one existing draft (`APPEND` or `REPLACE`) guarded by `baseDraftRevision`.
   Appended nodes get free names and ids, and literal node references in their expressions are rewritten.
3. Report format `orcon/n8n-import-report@1`: outcomes `CONVERTED`, `ADAPTED`, `NEEDS_REVIEW`,
   `UNSUPPORTED`; per-node findings with a JSON Pointer and stable codes; credential resolutions;
   behaviour differences; gates that block publication or activation. It carries no parameter, expression
   or secret value. Stored as codes and parameters; messages are rendered per request language.
4. Credentials: each referenced n8n credential links to a same-name, same-type credential of the project
   or becomes a credential with status `NEEDS_SECRET`; publishing a flow that references one fails until
   its secret is written.
5. Run semantics. A disabled node that is not a placeholder does not run: the items of its first input
   leave unchanged through its `main` port and it never dispatches an effect; a disabled trigger registers
   no address. The placeholder `orcon.unsupported@1` is always disabled and never passes items: in a test
   run an item that reaches it fails that node with the permanent error `UNSUPPORTED_NODE`; a published
   version cannot contain one.

## Consequences
An n8n user gets a flow and a report in one request; imported flows can be tested before every node is
rebuilt; the report is the same in the web, the API and the CLI. Very large migrations are split into
several requests until a long-running variant exists.

## Alternatives considered
Long-running import per ADR-0053 §7 as written (deferred: needs the operation framework and the input
stored until a worker claims it, for a CPU-only task that finishes in seconds); client-side creation with
`createFlow` plus `replaceDraft` (rejected: two requests, not atomic, no placeholder credentials);
placeholders that pass items through (rejected: a skipped email or charge would look like it ran);
refusing to run any flow with a placeholder (rejected: n8n users could not try a partly converted flow).
