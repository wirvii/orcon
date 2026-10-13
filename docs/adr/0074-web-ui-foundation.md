<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0074: Web UI foundation

Status: Accepted
Date: 2026-10-12
Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

## Context
The embedded React editor (ADR 0002) needs a component foundation, styling system, design tokens, i18n and UI quality gates before product screens are built. None has been adopted. The owner decided the direction in BL-007 (Q1–Q13) and in follow-up decisions P1–P3.

## Decision
1. Components: shadcn/ui source (MIT) copied into `@orcon/ui` and fully re-themed, on Radix Primitives (MIT). No stock shadcn look.
2. Styling: Tailwind CSS v4 (MIT), CSS-first, with Tailwind's default palette and scales reset; only Orcon tokens generate utilities.
3. Tokens: `@orcon/tokens`, W3C DTCG JSON compiled by Style Dictionary v4 (Apache-2.0) to CSS variables, a Tailwind theme, typed JS and JSON; light/dark × comfortable/compact. The exact brand navy is the dark-mode panel surface; red, amber and violet are status-only colors outside the brand palette, with violet reserved for attention states, primarily UNCERTAIN.
4. Fonts: Inter and JetBrains Mono (OFL-1.1) self-hosted through Fontsource and embedded in the binary; no external font requests.
5. Icons: Lucide (ISC). Final brand assets (symbol, wordmark, favicon, mascot exports) are delivered separately; until then the UI uses neutral placeholders behind stable `BrandMark` and `Mascot` components.
6. i18n: react-intl / FormatJS (BSD-3-Clause), ICU messages, English and Spanish in V1.
7. Quality gates: ESLint 9 and Stylelint token rules, Storybook 9 with Vitest browser tests and axe (axe-core MPL-2.0, development only, not distributed), Playwright visual regression in a pinned container, a token contrast test, story coverage.
8. The design rules live in the repository skill `orcon-ui` (`.agents/skills/orcon-ui/`, Agent Skills standard + mneme template). A byte-identical projection is generated into the tool configuration directories for clients that read skills from there (`make skill-sync`). `docs/design-system/README.md` summarizes and links to it.

## Consequences
New runtime dependencies are permissive and bundled by Vite into the binary; their notices are added to the third-party notices. CI gains UI jobs and a committed lockfile. Product screens may rely on the design system only after its rules are Accepted.

## Alternatives considered
- MUI / Chakra / Mantine: heavier theming layers and stock looks; harder to make fully Orcon.
- Headless only (Radix without shadcn): more work for the same result; shadcn source is ours to change.
- CSS Modules / vanilla-extract without Tailwind: viable, but the owner chose shadcn + Tailwind (Q1).
- Chromatic for visual review: proprietary SaaS; rejected for a community Apache-2.0 project.
- i18next: MIT and capable; FormatJS chosen for native ICU, Intl integration and pseudo-locales.
- Rules in docs/design-system instead of the skill: rejected by the owner in favor of a self-contained, portable skill.
