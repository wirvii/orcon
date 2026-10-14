---
title: Support matrix
description: The operating systems, databases, container and cluster tools, and storage that Orcon supports, and how well each is tested.
sources:
  - ADR-0083
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Support matrix

> This page describes behavior specified by ADR 0083 that is not implemented yet. The documentation site will generate this page from `docs/user/support-matrix.yaml`, the same file that drives the install tests, and a test will compare the two. Until then this page is written by hand from that file. If they differ, the file is right.

## Tiers

| Tier | What it means |
| --- | --- |
| 1 | The install test and the full test battery run on every release. |
| 2 | The binary is published. The install test runs when a CI runner is available. The full battery does not run. |
| Evaluation | Published, but not recommended for production. |

## Platforms

| Platform | Artifacts | Tier | Install test runner |
| --- | --- | --- | --- |
| Linux amd64 | Binary and image | 1 | `ubuntu-24.04` |
| Linux arm64 | Binary and image | 1 | `ubuntu-24.04-arm` |
| macOS arm64 | Binary | 1 | `macos-15` |
| macOS amd64 | Binary | 2 | An Intel runner if one exists; otherwise published only |
| Windows amd64 | Binary | Evaluation | `windows-2025` |
| Windows arm64 | Binary | Evaluation | `windows-11-arm` if it exists; otherwise published only |

The minimum system requirements are those of the Go toolchain that builds each release. The page for each version cites them.

## PostgreSQL

| Version | Status | Notes |
| --- | --- | --- |
| 14 | Supported | The community ends support on 2026-11-12. PostgreSQL 14 leaves the matrix with the first minor release published after 2027-05-12. |
| 15 | Supported | |
| 16 | Supported | |
| 17 | Supported | |

PostgreSQL 14 and 17 are tested on every pull request and every night. PostgreSQL 15 and 16 are tested in the release battery. PostgreSQL 18 is tested in the nightly battery and enters the matrix only after a release has tested it in the release battery. A PostgreSQL or Kubernetes version enters the matrix only when it is in the test matrix, and it leaves with a notice in the notes of a minor release.

## SQLite

SQLite is built in (the `modernc.org/sqlite` driver). It works with the `standalone` role only.

## Kubernetes and Helm

- Kubernetes: the three minor versions supported by the community on the release date.
- Helm: 3.14 or newer, and 4.x.

## Containers

- Docker Engine 24 or newer.
- Docker Compose v2.

## Object storage

- S3: tested with AWS S3 through its API. The tests in CI use an Apache-2.0 compatible server (SeaweedFS).
- Other S3-compatible services should work. They are not tested.

## Browsers

The supported browsers are those of the web application standards.
