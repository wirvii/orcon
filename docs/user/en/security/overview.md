---
title: Security overview
description: How Orcon protects your data, who can do what, and where to read the details.
sources:
  - ADR-0038
  - ADR-0042
  - ADR-0046
  - ADR-0048
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Security overview

> This page describes behavior specified by the accepted security design (ADR 0038 to ADR 0051) and by ADR 0083 that is not implemented yet. The slices that deliver each part remove this note.

This page is a map of Orcon's security model. Each section says what Orcon promises and sends you to the page with the details and the settings.

## No capability is held back

Every security capability described here ships in the Apache-2.0 engine: single sign-on, roles, audit, encryption and isolation. None is gated by an edition, a license key or a payment. A platform built on top of Orcon can only tighten policy through the same configuration that you use.

## The building blocks

| Area | What Orcon does | Read more |
| --- | --- | --- |
| Organizations | An instance holds organizations, and an organization holds projects. Organizations are hard boundaries: a member of one cannot see another, and every row of customer data carries its organization. | [Multiple organizations](multi-organization.md) |
| Sign-in | Email and password with a second factor (an authenticator app or a passkey), plus OIDC, SAML, LDAP and SCIM for organizations that want single sign-on. | [Authentication and SSO](authentication-and-sso.md) |
| Permissions | Roles at organization and project level, custom roles, and a separate permission to see execution data. Denied unless granted. | [Authorization](authorization.md) |
| Credentials | Secrets you save are never shown again. A flow uses a credential without being able to read it. External secret stores are supported. | [Credentials and secret stores](credentials-and-secret-stores.md) |
| Encryption | Layers of keys under a master key that you control. | [Encryption and keys](encryption-and-keys.md) |
| Audit | A tamper-evident log of who did what, exportable to your security tools. | [Audit and SIEM](audit-and-siem.md) |
| Connectors | Connector packages are signed and reviewed, and you choose the trust policy. | [Connector trust](connector-trust.md) |
| Network | Orcon makes no unsolicited outgoing connection. | [Network connections](../configuration/network-connections.md) |

## Instance administrators and organization data

The people who run the instance are separate from the people who own an organization. Instance roles (`instanceOwner` and `instanceAdmin`) control the master key, connector trust, the lifecycle of organizations, limits and backups. They do not give access to the data of an organization. To see it, an instance administrator must join the organization explicitly, and that is audited.

## Policy in layers

Security settings come in three layers: the engine defaults, then the floors set by the instance, then the settings of each organization. An organization can only tighten what the instance sets. A setting marked `locked` is read only from the configuration file or the environment at startup, and cannot be changed in the web interface or through the API.

## Release integrity

Every release is signed and comes with build provenance. See [Verifying releases](../install/verifying-releases.md).

## What to do next

- Apply the [hardening checklist](hardening-checklist.md) to your installation.
- To report a vulnerability, see [Reporting vulnerabilities](reporting-vulnerabilities.md).
