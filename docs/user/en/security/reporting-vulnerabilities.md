---
title: Reporting vulnerabilities
description: How to report a security problem in Orcon privately, and what to expect after you do.
sources:
  - ADR-0081
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Reporting vulnerabilities

If you found a security problem in Orcon, report it privately. Do not open a public issue, pull request or discussion.

## How to report

Use GitHub's private vulnerability reporting: <https://github.com/wirvii/orcon/security/advisories/new>.

Include:

- the affected version or commit,
- your configuration (runtime role and database),
- the steps to reproduce the problem,
- the impact you observed,
- any fix you suggest.

## What to expect

- We acknowledge your report within 5 business days.
- We send an initial assessment within 10 business days.
- We agree on a fix and a disclosure date with you. By default we publish within 90 days of the report.
- We credit you in the advisory, unless you prefer otherwise.

## Which versions get fixes

Before version 1.0, only the latest minor release receives security fixes.

## What is in scope

The Orcon source code, the release binaries and the container image `ghcr.io/wirvii/orcon`. If the problem is in a third-party service that Orcon connects to, report it to that service.

## Release integrity

If a signature check fails for an official release, or you find a signature that should not exist, report it the same way. To check releases yourself, see [Verifying releases](../install/verifying-releases.md).
