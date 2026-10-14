---
title: Connector trust
description: How Orcon decides which connector packages can be installed, what the seals mean, and when an instance contacts a remote catalog.
sources:
  - ADR-0033
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Connector trust

> This page describes behavior specified by ADR 0033 and ADR 0083 that is not implemented yet. The slices that deliver the connector catalog remove this note.

A connector is code that talks to another service on behalf of a flow. Orcon lets you decide which connectors can run, and it shows where each one comes from.

## Seals

Every package has exactly one seal.

| Seal | Meaning |
| --- | --- |
| `OFFICIAL` | Written by Orcon and signed by its release pipeline. Connectors built into the binary are official. |
| `VERIFIED` | Reviewed by Orcon and signed by the catalog service after an approved review. |
| `UNREVIEWED` | Everything else: packages from a private catalog (shown as "signed by" the catalog name), local files, and the output of the converter. |

Only the Orcon catalog holds the keys for the first two seals. The public converter and local tools never produce them.

## Trust policy

The policy is configuration, not code. It has three profiles.

| Profile | What it allows |
| --- | --- |
| `OPEN` (the default) | All seals. The Orcon public catalog plus the private catalogs that an administrator adds. Unsigned packages and local files stay off until the administrator allows them. |
| `STRICT` | Only `OFFICIAL` and `VERIFIED` packages, only from the Orcon public catalog, no unsigned packages and no local files. It is locked. |
| `CUSTOM` | Your own choice of seals, catalogs and the two switches for unsigned packages and local files. |

The policy is read from `connectors.trust`. When it is `locked`, it can be read only from the configuration file or the environment at startup, and the web interface and the API show it as read-only. The policy is checked at install, again at publication and again at every dispatch. If you tighten it, installed packages that no longer conform are blocked by policy: their nodes fail validation and dispatch with an explicit message.

All profiles are in the same Apache-2.0 binary and free for every installation. No capability depends on an edition, a license key or a payment.

## What you approve when you install

At install the administrator sees the seal, the publisher, the license, how far the safety claims of each operation are verified, the network domains, the credential types, any raw access to credentials, and the sensitive fields. The approval is stored with a digest of those permissions. If an update changes them, a new approval is needed, and until then the nodes stay on the previous version.

## Verification

Catalogs follow The Update Framework (TUF), so a mirror is only a transport and cannot forge a package. Installing runs a full TUF update, checks lengths and hashes, and checks the policy. Workers verify the package digest before they load it. Expired freshness metadata blocks new installs and updates, except in offline mode.

## Revocation

A catalog can revoke a package version, as `CRITICAL` or `ADVISORY`.

- `CRITICAL`: dispatch of the affected node is blocked. Flows stay published and active, and each dispatch fails with `ConnectorRevoked`, naming the connector, the version, the reason and the version to upgrade to. Each affected organization gets an alert and a `connector.revoked` event, to its Owners, Admins and the owners of the affected projects, until acknowledged. An administrator can record a supervised override for one package and version, with a reason and an expiry. It is audited.
- `ADVISORY`: a warning in the editor and the dashboard. Dispatch continues.

No setting downgrades `CRITICAL` to a warning.

## When an instance contacts a catalog

An instance that has never installed a package from a remote catalog never contacts one. Orcon fetches the catalog metadata every time you install or update from a remote catalog. While at least one package that an administrator installed from a remote catalog remains installed, it also refreshes that metadata every `metadataRefresh` (6 hours by default). Setting `metadataRefresh` to `0` stops the periodic refresh.

Packages preinstalled in an image, or installed from a bundle file, never schedule a refresh. Their revocation information is as fresh as the bundle or release that brought them.

## Offline install

`orcon connector install --bundle <file.orconbundle>` installs a package together with its full metadata chain, with no network. Expired freshness metadata is accepted only with `--offline` and the administrator's confirmation, and the web interface says that revocation information is only as fresh as the bundle.

## Private catalogs

You can run your own catalog. `orcon catalog init`, `orcon catalog add`, `orcon catalog sign` and `orcon catalog serve` create, fill, sign and serve it as static files. Packages from it carry the `UNREVIEWED` seal, labelled with your catalog's name.
