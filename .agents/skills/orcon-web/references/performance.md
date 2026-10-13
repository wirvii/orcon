<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Performance

Status: Proposed — pending owner approval

This document defines the performance budgets of the web application and how each one is measured: initial JavaScript size, first useful screen, the 500-node editor and long lists. The phone and frame-rate budgets are measured in CI with Chromium CPU and network emulation; this is a reproducible approximation, not a measurement on a physical phone or display (ADR 0078, maintainer decision of 2026-10-12). The visual side of the canvas is defined by `orcon-ui` ([canvas](../../orcon-ui/references/canvas.md#performance)). Rule IDs (`PF-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Initial load

- **PF-1** Initial JavaScript (the entry chunk and every chunk it imports statically, from the Vite manifest) is at most 250 kB gzip (250,000 bytes), checked by size-limit on every pull request.
- **PF-2** The editor (`@xyflow/react`, editor store, node configuration renderer) and other heavy screens load only through route code splitting or `lazy()`; `check-initial-chunks.mjs` fails if they reach the initial chunks.

## First useful screen

- **PF-3** First useful screen: navigation start → `performance.mark('orcon:first-useful')`, set by `markFirstUsefulScreen` when the route's main content (not a skeleton) is painted. Median of 5 cold loads against the real binary: < 1500 ms desktop (no throttling, 1440×900) and < 3000 ms mid-range phone over 4G. The phone profile is emulated in Chromium on the CI runner: CPU 4× slower, 150 ms latency, 1.6 Mbps down, 750 Kbps up, 390×844. This emulation is an approximation, not a measurement on a physical phone (maintainer decision, 2026-10-12). Every main route sets the mark and joins the test.

## Editor

- **PF-4** Editor with 500 nodes, using `perf/fixtures/reference-flow-500.json` (seeded generator: 500 nodes, 560 edges, 12 loop-back edges): open < 1000 ms (navigation start → `orcon:editor-ready`); 3 s each of scripted pan, zoom and node drag with at most 5 % of frames longer than 25 ms and no frame longer than 50 ms — this is the measured meaning of "60 fps"; median of 3 runs; desktop profile on the CI runner. This is an approximation: a headless browser on a CI runner does not reproduce a real GPU or display (maintainer decision, 2026-10-12).

## Lists

- **PF-5** A list that can exceed 100 rows uses the virtual mode of the design-system data table and cursor pagination. With 10,000 rows: at most 60 rows in the DOM, first render < 100 ms, scrolling to the end without a frame longer than 50 ms.
- **PF-7** Lists request at most 200 items per page; no request per row.

## Rendering

- **PF-6** Render discipline: memoized node and row components, narrow store and query selectors, stable callbacks for canvas handlers, no per-frame React state for edges.

## Changing a budget

- **PF-8** Budgets are maintainer decisions: tightening is allowed in review; loosening needs the maintainers' approval recorded in a spec. A budget is never loosened to make an intermittent CI failure pass.
