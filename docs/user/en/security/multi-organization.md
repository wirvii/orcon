---
title: Multiple organizations
description: How Orcon keeps the data of one organization apart from another on the same instance, and what each database can guarantee.
sources:
  - ADR-0038
  - ADR-0046
  - ADR-0047
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Multiple organizations

> This page describes behavior specified by ADR 0038 and ADR 0046 that is not implemented yet. The slices that deliver organizations and isolation remove this note.

One Orcon instance can serve several organizations. An organization is a hard boundary: its flows, credentials, executions and keys are invisible to the others.

## Organizations, projects and people

The structure is instance, organization, project, resources. A new single-organization install creates an organization called `default`, and the web interface hides the choice of organization until a second one exists. Each organization has a default project. A person has one account on the instance and a membership in each organization they belong to. A user without an active membership cannot see the organization or even know that it exists.

An organization is created by instance administrators, unless the instance enables `orgs.selfService`. It can be suspended: its triggers stop, sessions are denied and the data is kept. Deleting it starts a 30-day grace period, during which an instance administrator can cancel. After that its keys are destroyed and its rows and blobs are removed in batches.

## How isolation is enforced

Every row of customer data belongs to one organization, and the database refuses a row that points to a parent in another organization. On top of that, Orcon applies four independent checks:

1. **The data layer.** All queries go through one layer, and a query for organization data cannot be written without the organization.
2. **Static checks in the build.** The build fails if code queries the database outside that layer or if an organization table is read without being restricted to its organization.
3. **Row-level security in PostgreSQL.** Each table enforces the organization in the database itself. The runtime role cannot bypass it and owns no tables.
4. **An isolation test suite** that tries to read, change and reference one organization's resources from another, through every route and every store method, on SQLite and on PostgreSQL. It is part of the security release gates.

Other boundaries follow the same rule: caches, blob paths, the webhook route index and sub-flow calls never cross organizations.

## What SQLite can and cannot do

SQLite has no row-level security, so on SQLite only checks 1, 2 and 4 apply. Creating a second organization on SQLite shows a warning in the web interface, is logged and is audited. For more than one organization, use PostgreSQL. Running one instance for each customer is also possible. When an instance has two or more organizations on PostgreSQL, Orcon refuses to start, and refuses to create the second organization, if the runtime role can bypass row-level security, is a superuser, owns tables, or if an organization table lacks the policy. With one organization it warns.

The row-level security defends against a missing or wrong filter in Orcon's own queries. It does not defend against arbitrary SQL run as the application role.

## PostgreSQL roles

Orcon uses an owner role that runs migrations, an application role for normal work and an engine role for a few internal functions. If your database user may create roles, Orcon creates the other two. If not, `orcon db setup-roles` prints the SQL for your database administrator.

## Settings per organization

Authentication sources, security policy, audit sinks, secret stores, limits and keys are per organization. An organization can only tighten the floors that the instance sets. See [Authentication and SSO](authentication-and-sso.md).
