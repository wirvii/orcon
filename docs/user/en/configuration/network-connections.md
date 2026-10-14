---
title: Network connections
description: Every reason Orcon opens an outgoing connection, and how to know that a fresh install opens none by itself.
sources:
  - ADR-0083
  - ADR-0033
  - ADR-0044
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Network connections

> This page describes behavior specified by ADR 0083 that is not implemented yet. The slice that delivers the registry of outgoing connections removes this note. When it is delivered, the table of purposes below is generated from that registry, and this page is replaced by the generated version.

Orcon never sends usage data or an installation identifier to the project or to anyone else. This page lists every reason an Orcon process opens an outgoing connection, so that you can write a firewall rule or check a claim.

## The guarantee

Every outgoing connection has one of three causes:

1. **Your configuration names the destination.** A database, a key management service, a mail server, a telemetry endpoint, an S3 bucket, an audit sink, a secret store, an identity provider, or the update check, once you turn it on.
2. **Your flows, credentials and triggers.** A flow that calls a service connects to it.
3. **An administrator's explicit action.** Installing or updating a connector from a remote catalog. While at least one package installed that way remains installed, Orcon also refreshes that catalog's metadata from time to time.

A fresh install, with the default settings, makes no unsolicited outgoing connection.

## The purposes

Each component that can open a connection is registered with one purpose from this closed list, and with the key or action that turns it on.

| Purpose | What it is for | What turns it on |
| --- | --- | --- |
| `STORE` | The PostgreSQL database | `store.driver: postgres` and its connection string |
| `KMS` | A key management service for the master key | A `keys.master.provider` other than `file` or `env` |
| `SMTP` | Sending email | The mail server settings |
| `OTLP` | Exporting metrics, traces or logs over OpenTelemetry | `observability.otlp.*` |
| `BACKUP_TARGET` | Writing backups to S3-compatible storage | `backup.target` set to an `s3://` address |
| `BLOB_S3` | Storing blobs in S3-compatible storage | `blobs.backend: s3` |
| `SIEM` | Sending audit events to a security information system | An audit sink that you configure |
| `SECRET_STORE` | Reading secrets from an external store | A secret store that an organization configures |
| `IDENTITY_PROVIDER` | Signing in through OIDC, SAML or LDAP | An authentication source that an organization configures |
| `UPDATE_CHECK` | Looking for a newer version | `updateCheck.enabled: true` |
| `CATALOG` | Installing from a remote connector catalog, and refreshing its metadata | An administrator installs a package from a remote catalog |
| `FLOW` | What your flows do | Your flows, credentials and triggers |

Database connections to SQLite, and anything on the loopback address, are not outgoing connections.

## The update check

It is off by default (`updateCheck.enabled: false`). You can turn it on only in the configuration file, the environment or a flag; there is no switch in the web interface. When it is on, about every 24 hours (with a random variation of one hour) Orcon sends a plain `GET` request to the public list of releases (`updateCheck.url`) with the user agent `orcon`, without a version, cookies, query parameters or any data about your installation. It compares the answer locally and tells instance administrators when a newer stable version exists. The server that you ask can see the address your request comes from.

## The connector catalog

An instance that has never installed a package from a remote catalog never contacts one. Packages that come preinstalled in the image, or that you install from a bundle file, never schedule a refresh. Their revocation information is only as fresh as the bundle or the release that delivered them.

After an administrator installs a package from a remote catalog, Orcon refreshes that catalog's metadata every `connectors.trust.metadataRefresh` (6 hours by default) for as long as any such package stays installed. The refresh carries no installation identifier. Setting it to `0` turns the refresh off, and then revocations arrive only when you update by hand. See [Connector trust](../security/connector-trust.md).

## Installers

The one-line installers contact `github.com`, `objects.githubusercontent.com`, and the server of the Sigstore trust root that cosign uses. See [Install the single binary](../install/binary.md).

## How this is tested

Automated tests check the guarantee on every release candidate, after publishing and every night:

- A packet capture of the release image on an isolated network, through a fresh install, the first run, loading the web interface, running a sample flow and 15 minutes of idle time, fails if the Orcon process starts a single connection to an address that is not loopback or the test harness.
- A browser request log checks that the web interface asks for nothing from another origin: no external fonts, scripts or images.
- An in-process run with the clock moved forward 48 hours, with the default settings, records no outgoing connection attempt. The same test checks that each key that enables a purpose enables exactly that purpose, including the catalog cases above.
