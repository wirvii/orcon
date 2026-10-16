---
title: Importing n8n flows
description: Import n8n workflow files into Orcon from the web, the API or the command line, and read the import report.
sources:
  - ADR-0007
  - ADR-0037
  - ADR-0051
  - ADR-0084
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Importing n8n flows

n8n is a trademark of its owner; Orcon is an independent project. This guide names n8n only to say what it is.

> This guide describes the behavior specified for the import of n8n workflows. A feature is available only in versions that ship it. The reference of every code in the report is [n8n import report](../../../reference/n8n-import-report.md).

You export your workflows from n8n as JSON files and give them to Orcon. Orcon converts each workflow into a flow, creates it inactive and gives you a report that says what carried over, what changed and what you must do before the flow can be published or activated. Nothing is replaced silently. Importing never runs anything and never reads a secret.

## What you can import

Each file can hold:

- one workflow, as n8n exports it;
- one workflow inside a `workflow` member, or the double wrapper of the n8n template library;
- a list of workflows, as the export of all workflows produces;
- a page of the n8n public API, `{ "data": [ ... ] }`;
- the fragment n8n puts on the clipboard when you copy nodes in its editor.

An item that is not a workflow does not stop the import: it is skipped, and the report says its position and why.

## Import from the web

1. Open a project and choose **Import from n8n** in the chooser that creates a flow (or press `I`).
2. Paste the JSON, or choose one or more `.json` files, or drop them on the page. You can give up to 20 files and 8 MiB in total.
3. Orcon shows a report for each workflow before it creates anything: how many nodes are ready, adapted, to review and without an equivalent, how many nodes never send twice, the differences of behavior, and what blocks publishing or activating.
4. For each credential the workflow uses, choose **Create pending** (the default) or an existing credential of the same type. If you cannot read credentials you see only that a pending credential will be created.
5. Choose **Import and open** for one workflow, or **Import N flows** for several. For one workflow the editor opens on the new flow; for several you get a list with a link to each flow and the reason for each skipped one.

An imported flow shows a strip at the top of the editor, "Imported from n8n", with the number of nodes to review, the nodes without an equivalent and the credentials without a secret, and a link to the stored report. The numbers come from the current draft, so they go down as you fix things.

When you paste n8n nodes on the canvas of a flow you have open, Orcon converts them and renames any node whose name or id is already used in the flow. Expressions that refer to a renamed node are rewritten to the new name.

## Import with the API

`POST /api/v1/projects/{projectId}/imports` imports files from the request body. The request needs an `Idempotency-Key` header and the permission to import flows in the project.

- `sources` holds from 1 to 20 items, each with an optional `name` and the JSON `content`.
- `target` is `{ "kind": "NEW_FLOWS" }`, or the draft of an existing flow with `{ "kind": "EXISTING_FLOW", "flowId", "baseDraftRevision", "mode" }`. `mode` is `APPEND`, which adds the nodes to the right of the existing ones, or `REPLACE`, which replaces the document and keeps the id and name of the flow. An existing flow accepts one workflow. If the draft changed since `baseDraftRevision` the request fails with `412` and gives the current revision.
- `credentialChoices` optionally maps a credential `key` of the report to an existing credential of the project, or to a pending one.

The answer is `201` with the `Import` resource: totals, and for each workflow its status, its flow and its report. Everything is saved together or nothing is: if the request fails, no flow, draft or credential remains, and repeating it with the same key creates the flows once. `GET /api/v1/imports/{importId}` reads the import again; the messages of the report follow the `Accept-Language` header (English or Spanish).

`POST /api/v1/projects/{projectId}/n8n-converter/evaluate` converts one workflow without saving anything and returns the fragment and the report. It accepts `reservedNodeNames` and `reservedNodeIds` so the new nodes do not collide with the ones you already have.

## Convert on the command line

`orcon n8n convert <file...>` converts without a network connection, with the same converter and the same report as the server:

```
orcon n8n convert export.json --out-dir flows --report report.md --report-json report.json
```

It writes one `orcon/flow@1` file for each workflow in `--out-dir`, a readable report with `--report` and the JSON report `orcon/n8n-import-report@1` with `--report-json`. Use `--id` to choose the flow id. The exit code is 0 when the conversion finished, even if some nodes became placeholders, 2 for a wrong use of the command and 5 for an input that is not valid. The commands that send files to a server (`orcon n8n import` and `orcon imports get`) arrive with the network client of the command line.

## What the report tells you

Each node ends in one of four outcomes:

- **Converted**: it runs the same way.
- **Adapted**: it runs, and the report states one difference.
- **To review**: a person has to look at it. It is marked unsafe (for example an HTTP request that changes data), it uses a credential without a secret, or one of its expressions has findings.
- **No equivalent**: Orcon has no node for it. See below.

The report also lists differences of behavior for the whole workflow, for example that the workflow was active in n8n and is inactive here, that branches run in order, or that pinned data was not imported. The workflow keeps the name it had in n8n; if another flow of the project has it, a number is added.

Expressions are copied unchanged and checked, never run. A finding names the field of the node that holds the expression.

## Credentials

Secrets are never read, saved or shown. For each credential a node uses:

- if the project already has a credential with the same name and the same type, the node uses it;
- otherwise Orcon creates a pending credential with that name and type, which has no secret and the status `NEEDS_SECRET`;
- if you cannot create credentials, the node keeps the name and the report says that someone who can must create it.

You can instead choose any existing credential of the same type at import time.

A flow that uses a credential without a secret cannot be published until you write the secret. A test run that reaches the node fails with an error that names the credential. If the credential is for generic HTTP authentication, the report also asks you to set the hosts it may be sent to.

## Nodes without an equivalent

A node Orcon cannot convert becomes a placeholder. The placeholder keeps the original node (without any value that looks like a secret), its connections and its position, and it is always disabled.

- You can test the flow. When an item reaches a placeholder, the test stops at that node with the error `UNSUPPORTED_NODE`, which names the node and its n8n type, and the run ends as the node's error setting says. Branches that do not reach it run normally. A placeholder never lets items through, so a skipped email or charge never looks as if it ran.
- You cannot publish the flow while it has a placeholder.
- If the node was a trigger, the flow has no trigger that can start it, and the report says so.

## Disabled nodes

A node that was switched off in n8n stays disabled. When the flow runs, a disabled node does nothing: the items that enter it leave unchanged through its main output, and it never sends anything outside. A disabled trigger does not register its address when the flow is activated.

## Limits

- 8 MiB for the request, 20 files and 50 workflows per import. A migration larger than that is imported in several requests.
- At most 2,000 nodes and 10,000 connections per workflow, and a converted document of at most 8 MiB. A workflow over these limits is skipped and the others are imported.
- Converting a workflow has a budget of 10 seconds. At most two conversions run at the same time in an organization; a request over that limit is answered with `429` and should be repeated after two seconds.
- The paste conversion in the editor accepts up to 200 nodes.

## The limit of the secret detector

Exports from n8n do not contain the secrets of credentials. As a second protection Orcon checks every converted node for values that look like a secret, and a node that has one becomes a placeholder, with the value replaced by `NEEDS_SECRET`. The check is heuristic: a token stored in a field with a neutral name might not be recognized. Look at your exported files before you share them, and do not paste files you would not store.

The imported report and the stored import never contain a parameter value, an expression or a secret. The original file is not stored.
