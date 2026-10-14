---
title: Migrating from n8n, overview
description: What carries over from n8n to Orcon, what changes, and what you have to review before you activate imported flows.
sources:
  - ADR-0007
  - ADR-0018
  - ADR-0022
  - ADR-0023
  - ADR-0035
  - ADR-0037
  - ADR-0051
  - ADR-0071
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Migrating from n8n, overview

n8n is a trademark of its owner; Orcon is an independent project. This guide names n8n only to say what it is.

> This guide describes the behavior specified by the accepted decisions of the project. A feature is available only in versions that ship it; the pages of this section are written ahead of the importer and are corrected when it ships.

Orcon aims at familiar interaction and at importing n8n flows. It does not promise to be a drop-in replacement. Whatever differs is shown to you before you activate anything, and nothing is replaced silently.

## What carries over

- **Flows.** The importer reads n8n workflow exports and classifies each part as supported exactly, supported with an adaptation, or not supported. A flow is fully importable when every node and operation is mapped, every expression is accepted, every trigger is mapped and the Code node languages and libraries it uses are available.
- **Expressions.** Orcon uses n8n-style `{{ }}` expressions written in JavaScript with the same `$` names. Imported expressions are copied unchanged, and the importer reports the ones it cannot accept. Expressions run in a sandbox with no network, file or credential access, and with time and memory limits.
- **Webhook paths.** Imported webhook triggers keep their exact paths and their response behavior, so moving your traffic means pointing a domain at Orcon. New flows use Orcon addresses.
- **Core nodes.** Every n8n logic and utility node type is mapped to an Orcon node. An option with no equivalent, such as the SQL query mode of the Merge node, is reported.
- **Integrations.** Orcon chooses connectors from measured use in n8n's public template library. The goal of each release is that at least 90 % of those templates import completely, counting every template equally. The published coverage report says how close a release is. For anything without a connector, the generic HTTP node is the fallback.
- **AI nodes.** Every n8n AI node maps to an Orcon node, to an option of an Orcon node, to a connector kind, or is reported as not supported with a suggestion.

## What changes

- **Imported flows stay inactive.** You review them first. A write call that Orcon cannot prove safe to repeat, such as an HTTP request that changes data, is imported marked as unsafe, and the flow cannot be activated until you review every marked node.
- **Secrets are not imported.** Credentials arrive as placeholders with their name and type but no secret value. You enter the secrets once in Orcon. Users are imported as invitations; passwords and second factors are never imported.
- **Some things are reported, not converted.** A construct the importer cannot map is listed in the import report with the node and the field. An example is the LangChain Code node, which runs arbitrary code of that framework.
- **Waiting for a result has a time limit.** A webhook that waits for the result of a flow answers after 300 seconds on an address that imitates n8n, and after 30 seconds on a native address, with the execution id, and the execution continues. n8n has no such limit; the import report lists this difference.
- **Time is recorded once per step.** `$now`, `$today` and `Date.now()` return the time recorded when the step started, so they do not differ between items of the same step. In n8n they differ by microseconds.
- **Community nodes are not run as they are.** Orcon has no compatibility layer for n8n community nodes. A public converter turns community nodes written in the declarative style into Orcon connectors, and reports the ones that need code.
- **Commands and files on the host are off by default.** The nodes that run commands or read and write files on the host exist, and an instance administrator must enable them with an explicit allowlist.
- **Imported AI agents redact data by default.** An imported agent that has a public entry point also asks for Orcon approval for its sensitive tools. The import report lists these differences.
- **Not covered yet.** The n8n Data Table node has no equivalent; the importer reports it as not supported.

## How a migration usually goes

1. Import your flows and read the import report.
2. Enter the secrets of the credential placeholders and send the user invitations.
3. Review each flow, in particular the nodes marked as unsafe and every reported difference.
4. Test the flows with the test run of the editor.
5. Activate the flows and, when you are ready, point your domain at Orcon.

The other pages of this section, which go deeper into each step, are written together with the importer.
