# Orcon — Product Vision

Status: **Accepted** | Owner: Wirvii | Version: 0.1

## Mission

Build a genuinely open-source, self-hostable, visual workflow automation and orchestration platform that users of n8n can adopt with minimal friction. Orcon is an independent implementation, not an n8n fork. Its core is a robust Go execution engine; the visual editor, integrations, API, and AI agents build on that engine.

## Intended users

1. Individuals and teams building visual automations.
2. Developers needing programmable, observable, durable orchestration.
3. Companies embedding a workflow editor or runtime into their own products.
4. SaaS providers offering a hosted or modified Orcon without commercial licensing restrictions.

## Product promises

- Apache-2.0 source code: users may modify, redistribute, fork, embed, self-host, or commercialize the software under the license.
- A familiar visual workflow authoring experience using React Flow.
- A single distributable multi-role Go binary embedding the built web UI.
- Local standalone deployment without separately installed services; distributed deployment backed by PostgreSQL.
- Explicit, verifiable execution semantics and recovery rather than vague exactly-once claims.
- Vendor-neutral integrations, AI model providers, and MCP tooling.
- A transparent n8n migration experience: classify imported nodes as supported, adapted, or unsupported; never silently change behavior.

## Non-goals

- Monetization or proprietary enterprise feature gates.
- Copying n8n implementation, branding, or restricted source.
- Shipping every n8n integration in the initial release.
- Treating a successful import as proof of behavioral equivalence.
- Building a second runtime specifically for AI agents.

## Success criteria (proposed, not yet quantified)

A new user can install Orcon and open the editor with one binary; create, execute and debug basic workflows; and import a documented subset of n8n workflows with explicit compatibility reporting. A contributor can fork and commercially embed Orcon without buying a license. Engine reliability must be demonstrated by failure-injection and recovery tests before production claims.

## Delivery model

The product owner and architecture collaborators produce product specifications, ADRs, brand assets, design system, and agent contracts. Implementation agents own code and tests, subject to reviewed specifications and quality gates.

## Open questions

Target n8n compatibility baseline; exact node and expression priorities; workflow definition schema; failure semantics; multi-tenant isolation model; resource limits; plugin sandbox; initial release milestones. None are implicitly approved by this document.
