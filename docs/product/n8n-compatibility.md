# n8n Compatibility Strategy

Status: **Proposed** | Version: 0.1

## Goal

Enable n8n users to migrate workflows with minimal rework, without copying protected source or pretending all workflows are compatible.

## Import pipeline

1. Parse exported n8n workflow definitions into a versioned intermediate representation.
2. Inventory nodes, versions, credentials references, expressions, connections, triggers, execution settings, and embedded code.
3. Classify each construct: **exactly supported**, **supported with adaptation**, **unsupported**, or **requires manual validation**.
4. Produce a machine-readable and human-readable migration report.
5. Convert only validated constructs into Orcon's canonical workflow definition.
6. Require user review of credentials, side effects, scheduling and trigger behavior before enabling an imported workflow.
7. Run differential tests with sanitized fixtures when feasible.

## Compatibility dimensions

- Node identifiers, parameters and versioned contracts.
- Trigger/webhook scheduling and activation behavior.
- Expression syntax, item linking, item shape and error handling.
- Branch ordering, joins, loops, waits, retries and timeouts.
- Credentials mapping without exporting secrets.
- Code nodes and community extensions.
- Agent and MCP nodes, model providers and approvals.

## Acceptance constraints

No silent node substitution, discarded connections, leaked credentials or automatic activation of uncertain workflows. Migration coverage must be reported against a declared n8n version and public test corpus. Importing JSON alone is not considered compatibility.

## Open decisions

Target n8n release; initial 20–50 nodes; expression runtime strategy; licensing review of fixture sources; compatibility scoring; regression fixture catalog.
