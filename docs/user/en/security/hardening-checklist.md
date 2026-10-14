---
title: Hardening checklist
description: A checklist to review before an Orcon installation holds real data.
sources:
  - ADR-0083
  - ADR-0042
  - ADR-0046
  - ADR-0048
  - ADR-0033
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Hardening checklist

> This page describes behavior specified by the accepted security design (ADR 0038 to ADR 0051) and by ADR 0083 that is not implemented yet. Some items refer to commands and settings that are delivered by later slices. Each item names the page that explains it.

Go through this list once before the installation holds real data, and again after major changes. Orcon's defaults are already safe: the listener is on the loopback address and metrics, exporters and the update check are off. The list is about what you add.

## Installation

- [ ] The release you installed was verified. [Verifying releases](../install/verifying-releases.md)
- [ ] You use a pinned version of the image or the chart, not a moving tag.
- [ ] `orcon config check` ends with no errors and no warnings you do not understand. [Configuration overview](../configuration/overview.md)
- [ ] Secrets are given in files, not in values, flags or the environment. Secret files can be read only by the account that runs Orcon. [Secrets](../configuration/secrets.md)

## Network

- [ ] Users reach Orcon only over HTTPS, and `publicUrl` is the HTTPS address. [TLS and reverse proxy](../configuration/tls-and-reverse-proxy.md)
- [ ] The operations listener (`ops.address`) is reachable only from your monitoring system, never from the internet.
- [ ] If a proxy sits in front of Orcon, `http.trustedProxies` lists exactly those proxies.
- [ ] You know what Orcon connects to. [Network connections](../configuration/network-connections.md)
- [ ] The update check stays off unless you want it. If you turn it on, you accept that the server you ask sees your address.

## Keys

- [ ] The recovery kit is stored away from this machine, and more than one trusted person knows where. [Master key and KMS](../configuration/master-key-and-kms.md)
- [ ] The master key provider is `file` with a file only Orcon can read, or a key management service. Avoid `env`.
- [ ] With a key management service, the key has deletion protection and key rotation turned on.
- [ ] You decided whether to configure a recovery public key (`keys.recoveryPublicKey`).
- [ ] Rotation of the organization keys is on its default schedule.

## Backups

- [ ] A backup schedule is configured, and `backup.keys.target` is a different place from `backup.target`. [Backup and restore](../operations/backup-and-restore.md)
- [ ] `backup.keys.recipients` lists the age recipients you hold, so that backups carry key material and do not raise the warning `KEY_MATERIAL_NOT_EXPORTED`.
- [ ] You have verified a backup by restoring it in a test environment.

## People and permissions

- [ ] Every organization requires a second factor (`require2FA`), or single sign-on, as its policy. [Authentication and SSO](authentication-and-sso.md)
- [ ] At most two break-glass users exist, each with a passkey.
- [ ] Operators do not have `execution:read-data` unless they need it. [Authorization](authorization.md)
- [ ] Few people are instance owners or instance administrators.
- [ ] Service accounts and API keys are limited to the permissions they need.

## Isolation

- [ ] With more than one organization, you use PostgreSQL, not SQLite. [Multiple organizations](multi-organization.md)

## Connectors and credentials

- [ ] The connector trust profile fits your policy. Use `STRICT` and `locked` if only reviewed connectors may run. [Connector trust](connector-trust.md)
- [ ] The generic HTTP node has an explicit list of allowed hosts on every credential.
- [ ] Secrets that already live in a store of your own are referenced from there. [Credentials and secret stores](credentials-and-secret-stores.md)

## Audit

- [ ] At least one external audit sink is configured, so that removing the newest events can be detected. [Audit and SIEM](audit-and-siem.md)
- [ ] The alerts of the monitoring kit are loaded, in particular `OrconAuditVerificationFailed`, `OrconBackupMissing` and `OrconKeyUnwrapFailures`. [Alerts](../operations/alerts.md)
