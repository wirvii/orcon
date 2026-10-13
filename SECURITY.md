<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub: <https://github.com/wirvii/orcon/security/advisories/new>. Do not open a public issue, pull request or discussion.

Include the affected version or commit, the configuration (runtime role, database), steps to reproduce, the impact you observed and any suggested fix.

## What to expect

- We acknowledge your report within 5 business days.
- We send an initial assessment within 10 business days.
- We coordinate a fix and a disclosure date with you; by default we publish within 90 days of the report.
- We credit you in the advisory unless you prefer otherwise.

## Supported versions

Before version 1.0, only the latest minor release receives security fixes.

## Scope

Orcon source code, release binaries and the `ghcr.io/wirvii/orcon` container image. Vulnerabilities in third-party services that Orcon connects to should be reported to those services.

## Verifying releases

Every release is signed and comes with build provenance. See [docs/releases/verifying.md](docs/releases/verifying.md).
