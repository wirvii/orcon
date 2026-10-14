---
title: Versioning policy
description: How Orcon versions are numbered, how long the API stays stable, and which versions get fixes.
sources:
  - ADR-0081
  - ADR-0054
  - ADR-0015
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Versioning policy

> The parts of this page that refer to `orcon store schema`, the upgrade line in the release notes and the documentation site describe behavior specified by ADR 0083 that is not implemented yet.

## Release numbers

Orcon releases use semantic versioning: `MAJOR.MINOR.PATCH`. Each release has a changelog, grouped by type of change, and release notes. Publishing a release is a deliberate act of the maintainers. See [Verifying releases](../install/verifying-releases.md) for how releases are signed.

Orcon is in early development. Before version 1.0 nothing is production-ready, and only the latest minor release receives security fixes. How older versions are maintained after 1.0 is not decided yet. When it is, this page will say.

## Upgrading between versions

Each release says in one line whether you can upgrade without stopping anything: `Upgrade: rolling (floor vX.Y.Z)` or `Upgrade: requires full stop (floor vX.Y.Z)`. The floor is the oldest version that runs correctly against the database after the upgrade. You can skip minor versions when `orcon store schema plan` says `ROLLING_SAFE`; otherwise upgrade one version at a time, or stop everything. A breaking change to the database is split across two releases, so that the older version still works after the first. The procedure is in [Upgrade and rollback](../operations/upgrade-and-rollback.md).

## The API

The API is versioned in its path, as `/v1`.

- Inside a major version, only backward-compatible changes are made to `stable` and `beta` operations. Each operation declares its stability as `stable`, `beta` or `experimental`.
- A breaking change goes to a new major version, `/v2`. Breaking means, for example, removing or renaming something, adding a required field, narrowing accepted values or changing a type. The build compares the contract with the latest release and fails if a breaking change slips in.
- A deprecated item keeps working inside its major version, and is marked in the contract and with the `Deprecation` and `Sunset` response headers and a link to a guide.
- An old major version is supported for at least 12 months and at least two minor releases of Orcon, whichever is longer, after the next major version is stable. The release notes and an event announce it.

## Databases and platforms

A PostgreSQL or Kubernetes version enters the support matrix when it is in the test matrix, and leaves with a notice in the notes of a minor release. PostgreSQL 14 leaves with the first minor release published after 2027-05-12. See the [support matrix](matrix.md).

## The documentation

The documentation site has one version for each minor line of Orcon, a `next` version built from the main branch, and `latest`, which is a copy of the newest minor line. A patch release replaces the folder of its minor line. Old versions are not rebuilt.
