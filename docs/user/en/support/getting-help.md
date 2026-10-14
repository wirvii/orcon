---
title: Getting help
description: Where to ask questions, report bugs and propose changes, and what to include so that others can help you.
sources:
  - ADR-0081
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Getting help

Orcon is an open-source project, and its help channels are public.

## Before you ask

1. Search this documentation.
2. If an alert fired, read its section in [Alerts](../operations/alerts.md).
3. Collect what helps others see your situation. Some of these commands are delivered by later slices:

   ```sh
   orcon --version
   orcon config check
   orcon store schema status
   orcon health --ready
   ```

   `orcon config check` shows each setting and where it came from, with secrets shown as `[REDACTED]`. Read the output before you share it.

## Report a bug or propose a feature

Use the issue forms of the repository, <https://github.com/wirvii/orcon/issues/new/choose>. There is a form for a bug report ("something does not work as documented") and one for a feature request ("a new capability or a change in behavior"). Include the version, the runtime role and database, the steps to reproduce, and what you expected.

For anything bigger than a small fix, open an issue first, so that the maintainers can confirm the direction before you spend time on a change.

## Contribute

Contributions are welcome under the Apache License 2.0. Read the contributing guide at <https://github.com/wirvii/orcon/blob/main/CONTRIBUTING.md>. Everyone who takes part follows the code of conduct.

## Report a security problem

Never in a public issue. See [Reporting vulnerabilities](../security/reporting-vulnerabilities.md).
