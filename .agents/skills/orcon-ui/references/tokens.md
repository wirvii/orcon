# Design tokens

Status: Proposed — pending owner approval | Version: 1.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. The token source of truth is the package `@orcon/tokens` (`packages/js/tokens`); this document defines what that package contains and how code uses it.

## Architecture

Tokens are organized in five layers. A layer may reference only the layers listed before it.

| Layer | Source | Contains | May reference |
|---|---|---|---|
| Primitives | `src/primitives/color.json`, `src/primitives/{space,size,radius,typography,shadow,motion,z,breakpoint}.json` | Color ramps (green, cyan, slate/navy, red, amber, violet, white, black) and raw scales | Nothing |
| Brand | `src/brand.json` | The five exact brand colors, role `brand-exact` | Primitives |
| Semantic | `src/semantic/light.json`, `src/semantic/dark.json` | `bg`, `surface`, `fg`, `border`, `focus`, `action`, `status`, `canvas` | Primitives, brand |
| Density | `src/density/comfortable.json`, `src/density/compact.json` | Row and control heights, paddings, gaps | Primitives |
| Component | `src/component/*.json` | Per-component tokens (for example node width) | Semantic and density only |

1. Format: W3C Design Tokens Community Group (DTCG) JSON. Compiler: Style Dictionary v4 (`style-dictionary.config.mjs`).
2. Every semantic color token MUST be defined in both `light.json` and `dark.json`.
3. Every density token MUST be defined in both `comfortable.json` and `compact.json`.
4. Component tokens MUST NOT contain raw values; they reference semantic or density tokens only (enforced by `test/references.test.ts`).
5. Primitives and brand tokens are build inputs. They are not exposed as Tailwind utilities and components MUST NOT reference them.

## Naming

1. Token paths use dots in DTCG (`fg.muted`, `action.primary.bg-hover`, `status.attention.fg`).
2. CSS custom properties use the prefix `--orcon-` and the pattern `--orcon-<category>-<role>[-<variant>]`, with dots replaced by hyphens: `fg.muted` → `--orcon-fg-muted`; `action.primary.bg-hover` → `--orcon-action-primary-bg-hover`; `size.row` → `--orcon-size-row`; `duration.fast` → `--orcon-duration-fast`.
3. Tailwind utilities exist only for semantic, density and scale tokens, and their names mirror the token role: `bg-surface`, `bg-surface-raised`, `text-fg`, `text-fg-muted`, `text-fg-link`, `border-control`, `ring-focus`, `bg-action-primary`, `text-action-primary-fg`, `bg-status-attention`, `text-status-attention`, `h-control`, `h-row`, `px-control`, `gap-control`, `rounded-md`, `shadow-sm`.
4. Tailwind's default palette and scales are reset (`--color-*: initial;` and the equivalent for spacing, radius, shadow, font size and z). Utilities such as `bg-red-500`, `text-slate-600` or `p-7` do not exist.
5. Names describe a role, never a hue (`status.danger.fg`, not `red.fg`).

## Primitives

Color ramps (only the steps used by the semantic layer are listed; the source holds the full ramp):

| Ramp | Steps and values |
|---|---|
| green | 50 #F0FDF4, 300 #86EFAC, 400 #4ADE80, 500 #22C55E, 600 #16A34A, 700 #15803D, 800 #166534, 950 #052E16 |
| cyan | 50 #ECFEFF, 300 #67E8F9, 400 #22D3EE, 500 #06B6D4, 700 #0E7490, 800 #155E75, 950 #083344 |
| slate | 50 #F8FAFC, 100 #F1F5F9, 200 #E2E8F0, 300 #CBD5E1, 400 #94A3B8, 500 #64748B, 600 #475569, 700 #334155, 800 #1E293B, 900 #0F172A, 950 #020617 |
| navy | 950 #0B1120 |
| red | 50 #FEF2F2, 300 #FCA5A5, 400 #F87171, 700 #B91C1C, 800 #991B1B, 950 #450A0A |
| amber | 50 #FFFBEB, 300 #FCD34D, 800 #92400E, 950 #451A03 |
| violet | 50 #F5F3FF, 300 #C4B5FD, 700 #6D28D9, 950 #2E1065 |
| white / black | #FFFFFF / #000000 |

Brand (role `brand-exact`, references to primitives): `brand.green` = green-500, `brand.green-dark` = green-600, `brand.cyan` = cyan-500, `brand.navy` = slate-900, `brand.slate` = slate-500.

## Semantic colors

| Token | Light | Dark | Kind |
|---|---|---|---|
| `bg.app` | #F8FAFC | #0B1120 | Background |
| `surface.default` | #FFFFFF | #0F172A | Background |
| `surface.raised` | #FFFFFF (with shadow) | #1E293B | Background |
| `surface.sunken` | #F1F5F9 | #020617 | Background |
| `surface.inverse` | #0F172A | #F8FAFC | Background |
| `fg.inverse` | #F8FAFC | #0F172A | Text |
| `fg.default` | #0F172A | #F8FAFC | Text |
| `fg.muted` | #475569 | #CBD5E1 | Text |
| `fg.subtle` | #64748B | #94A3B8 | Text (restricted backgrounds) |
| `fg.disabled` | #94A3B8 | #64748B | Disabled text (exempt) |
| `fg.link` | #0E7490 | #22D3EE | Text |
| `fg.accent` | #15803D | #4ADE80 | Text |
| `action.primary.bg` | #15803D | #4ADE80 | Background / UI |
| `action.primary.bg-hover` | #166534 | #86EFAC | Background |
| `action.primary.fg` | #FFFFFF | #0F172A | Text |
| `action.danger.bg` | #B91C1C | #F87171 | Background |
| `action.danger.bg-hover` | #991B1B | #FCA5A5 | Background |
| `action.danger.fg` | #FFFFFF | #0F172A | Text |
| `border.default` | #E2E8F0 | #1E293B | Decorative |
| `border.control` | #64748B | #64748B | UI |
| `focus.ring` | #0E7490 | #67E8F9 | UI |
| `status.neutral.fg` / `.bg` | #475569 / #F1F5F9 | #CBD5E1 / #1E293B | Text / background |
| `status.active.fg` / `.bg` | #155E75 / #ECFEFF | #67E8F9 / #083344 | Text / background |
| `status.success.fg` / `.bg` | #166534 / #F0FDF4 | #86EFAC / #052E16 | Text / background |
| `status.warning.fg` / `.bg` | #92400E / #FFFBEB | #FCD34D / #451A03 | Text / background |
| `status.danger.fg` / `.bg` | #B91C1C / #FEF2F2 | #FCA5A5 / #450A0A | Text / background |
| `status.attention.fg` / `.bg` | #6D28D9 / #F5F3FF | #C4B5FD / #2E1065 | Text / background |
| `canvas.bg` | #F8FAFC | #0B1120 | Background |
| `canvas.grid` | #CBD5E1 | #1E293B | Decorative |
| `canvas.edge` | #64748B | #94A3B8 | UI |
| `canvas.port` | #64748B | #94A3B8 | UI |
| `canvas.edge-active` | #16A34A (brand-exact) | #22C55E (brand-exact) | UI |
| `canvas.selection` | #16A34A (brand-exact) | #22C55E (brand-exact) | UI |
| `canvas.node.border` | #CBD5E1 | #334155 | Decorative (a node is identified by its text) |

Rules:

1. `status.*` families map to domain states through [components.md#status](components.md#status). `status.attention` (violet) is reserved for UNCERTAIN, suspended and awaiting-a-person states.
2. `canvas.edge-active` and `canvas.selection` are the only semantic tokens that resolve to a brand-exact green; they are UI (non-text) tokens and MUST NOT be used for text or behind text.
3. `fg.disabled` is exempt from contrast minimums (inactive components, WCAG 1.4.3 exception) and MUST only be used for disabled content.

## Contrast pairs

1. Every foreground/background combination used by a component is declared in `src/pairs.json` as `{ fg, bg, kind, min, reason? }` with `kind` one of `text`, `ui`, `decorative`, `disabled-exempt`.
2. Minimums: `text` ≥ 4.5:1; `ui` ≥ 3:1. The large-text reduction is not used in V1. `decorative` and `disabled-exempt` pairs MUST carry a `reason`.
3. Ratios use the WCAG 2.x relative luminance formula and are computed for both themes by `test/contrast.test.ts`. A minimum MUST NEVER be lowered to make a test pass.
4. Current certified set: 71 pairs per theme, 0 failures. The authoritative, generated list is [contrast-report.md](contrast-report.md). Summary:

| Foreground | Background | Kind | Min | Light | Dark |
|---|---|---|---|---|---|
| `fg.default` | `bg.app` / `surface.default` / `surface.raised` / `surface.sunken` | text | 4.5 | 17.06 / 17.85 / 17.85 / 16.30 | 18.00 / 17.06 / 13.98 / 19.28 |
| `fg.muted` | same four | text | 4.5 | 7.24 / 7.58 / 7.58 / 6.92 | 12.68 / 12.02 / 9.85 / 13.59 |
| `fg.link` | same four | text | 4.5 | 5.12 / 5.36 / 5.36 / 4.89 | 10.42 / 9.88 / 8.09 / 11.16 |
| `fg.accent` | same four | text | 4.5 | 4.79 / 5.02 / 5.02 / 4.58 | 10.81 / 10.25 / 8.40 / 11.58 |
| `status.neutral.fg` | same four | text | 4.5 | 7.24 / 7.58 / 7.58 / 6.92 | 12.68 / 12.02 / 9.85 / 13.59 |
| `status.active.fg` | same four | text | 4.5 | 6.95 / 7.27 / 7.27 / 6.63 | 12.99 / 12.32 / 10.09 / 13.92 |
| `status.success.fg` | same four | text | 4.5 | 6.81 / 7.13 / 7.13 / 6.51 | 13.41 / 12.71 / 10.42 / 14.37 |
| `status.warning.fg` | same four | text | 4.5 | 6.78 / 7.09 / 7.09 / 6.47 | 13.06 / 12.38 / 10.15 / 13.99 |
| `status.danger.fg` | same four | text | 4.5 | 6.18 / 6.47 / 6.47 / 5.91 | 9.92 / 9.41 / 7.71 / 10.63 |
| `status.attention.fg` | same four | text | 4.5 | 6.79 / 7.10 / 7.10 / 6.49 | 10.20 / 9.67 / 7.92 / 10.93 |
| `fg.subtle` | `bg.app` / `surface.default` / `surface.raised` | text | 4.5 | 4.55 / 4.76 / 4.76 | 7.34 / 6.96 / 5.71 |
| `fg.inverse` | `surface.inverse` | text | 4.5 | 17.06 | 17.06 |
| `action.primary.fg` | `action.primary.bg` / `action.primary.bg-hover` | text | 4.5 | 5.02 / 7.13 | 10.25 / 12.71 |
| `action.danger.fg` | `action.danger.bg` / `action.danger.bg-hover` | text | 4.5 | 6.47 / 8.31 | 6.45 / 9.41 |
| `status.X.fg` | `status.X.bg` (neutral, active, success, warning, danger, attention) | text | 4.5 | 6.92, 6.99, 6.81, 6.84, 5.91, 6.48 | 9.85, 9.24, 10.62, 10.39, 8.51, 8.25 |
| `border.control` | `bg.app` / `surface.default` / `surface.raised` / `surface.sunken` | ui | 3.0 | 4.55 / 4.76 / 4.76 / 4.34 | 3.96 / 3.75 / 3.07 / 4.24 |
| `focus.ring` | same four | ui | 3.0 | 5.12 / 5.36 / 5.36 / 4.89 | 12.99 / 12.32 / 10.09 / 13.92 |
| `action.primary.bg` | same four | ui | 3.0 | 4.79 / 5.02 / 5.02 / 4.58 | 10.81 / 10.25 / 8.40 / 11.58 |
| `canvas.edge` / `canvas.port` | `canvas.bg` | ui | 3.0 | 4.55 | 7.34 |
| `canvas.edge-active` / `canvas.selection` | `canvas.bg` | ui | 3.0 | 3.15 | 8.26 |
| `canvas.selection` | `surface.default` | ui | 3.0 | 3.30 | 7.83 |

5. `fg.subtle` is **not** certified on `surface.sunken` (light 4.34 < 4.5). The pair is not declared, so the `pairs-declared` check rejects any component that uses it.
6. `border.control` in the dark theme on `surface.raised` is certified with a minimal margin (3.07:1). Any drift in either token fails the test; do not adjust one without recomputing.
7. `border.default`, `canvas.grid` and `canvas.node.border` are declared `decorative` (decorative dividers, WCAG 1.4.11 does not apply). `fg.disabled` is declared `disabled-exempt`.
8. No `text` pair may use a brand-exact hue token as foreground or background (`test/brand-exact.test.ts`).
9. Every semantic color token appears in at least one pair or is marked decorative with a reason (`test/coverage.test.ts`).

## Scales

| Scale | Values |
|---|---|
| Spacing (base 4 px) | 0, 1, 2, 3, 4, 5, 6, 8, 10, 12, 16 steps = 0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64 px |
| Radius | sm 4, md 8, lg 12, xl 16, full (9999) |
| Shadow (light theme) | sm, md, lg; dark theme uses border + raised surface and a faint shadow |
| Z layers | base 0, sticky 10, dropdown 20, overlay 30, modal 40, toast 50, tooltip 60 |
| Breakpoints | sm 640, md 768, lg 1024, xl 1280, 2xl 1536 |
| Font size / line height | xs 12/16, sm 13/18, base 14/20, md 16/24, lg 20/28, xl 24/32, 2xl 32/40, mono 13/20 |
| Font weight | 400, 500, 600, 700 |
| Font family | sans: Inter Variable; mono: JetBrains Mono Variable (self-hosted) |
| Icon size | 16, 20, 24 |

Radius usage: controls `md` (8); cards, panels, nodes `lg` (12); dialogs and sheets `xl` (16); pills and control nodes `full`.

## Density tokens

| Token | Comfortable | Compact |
|---|---|---|
| `size.row` | 40 px | 32 px |
| `size.control` | 36 px | 28 px |
| `space.control-x` | 12 px | 8 px |
| `space.gap` | 12 px | 8 px |
| `size.connector-icon` | 32 px | 24 px |
| `size.connector-icon-safe` | 20 px | 16 px |
| `size.node-width` (component) | 240 px | 208 px |
| `size.control-node-height` (component) | 40 px | 32 px |

`size.target-min` is 44 px under `@media (pointer: coarse)` and 24 px otherwise, independent of density ([foundations.md#density](foundations.md#density)).

## Motion tokens

| Token | Value |
|---|---|
| `duration.fast` | 120 ms |
| `duration.base` | 160 ms |
| `duration.slow` | 200 ms |
| `easing.standard` | cubic-bezier(0.2, 0, 0, 1) |
| `easing.enter` | cubic-bezier(0, 0, 0.2, 1) |
| `easing.exit` | cubic-bezier(0.4, 0, 1, 1) |

Under `@media (prefers-reduced-motion: reduce)` every `duration.*` resolves to 0 ms.

## Outputs

`pnpm --filter @orcon/tokens build` writes `dist/` (not committed):

| File | Content | Consumers |
|---|---|---|
| `tokens.css` | `:root` = light + comfortable; dark via `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {…} }` and `[data-theme="dark"]`; `[data-theme="light"]`; `[data-density="compact"]`; reduced motion sets durations to 0 ms | `@orcon/ui`, `apps/web`, Go-served pages |
| `tailwind-theme.css` | Tailwind v4 `@theme inline` that resets the defaults and maps utilities to `var(--orcon-*)` | `@orcon/ui` styles |
| `tokens.js` + `tokens.d.ts` | Typed token names and resolved values per theme and density | Canvas code and tests that need numbers |
| `tokens.json` | Resolved values | Go-served pages and tooling |

Go-served pages (hosted forms, the chat page, the approval page when served without the editor) link `tokens.css` and use `var(--orcon-*)` only; they follow the same theme attributes.

## Usage in code

1. Use Tailwind utilities generated from tokens, or `var(--orcon-*)` in CSS. Nothing else.
2. MUST NOT appear in `apps/web` or `packages/js/ui` code: hex, `rgb()`, `hsl()` or named colors; Tailwind arbitrary values (`w-[13px]`, `bg-[#…]`, `text-[length:…]`); Tailwind default palette or scale utilities; spacing outside the scale; inline `style` attributes.
3. The only inline `style` allowed is canvas positioning (node coordinates, viewport transforms) computed at runtime, each with an `eslint-disable` comment that states the reason.
4. Canvas code MAY import typed values from `@orcon/tokens` (`tokens.js`) only when a library API needs numbers (for example edge stroke width or node size for layout).
5. Never copy a resolved value into a component, story or test fixture; reference the token.

## Adding a token

1. Search the existing tokens first; reuse when the role matches.
2. Choose the layer: a role shared by several components is semantic; a measure that changes with density is a density token; anything specific to one component is a component token that references semantic or density tokens.
3. Define the value for both themes (semantic) or both densities (density).
4. Declare every pair the token takes part in, in `src/pairs.json`, with the right `kind` and minimum.
5. Run `pnpm --filter @orcon/tokens test`. It recomputes contrast, coverage, brand-exact and reference rules, and regenerates `contrast-report.md` in the skill.
6. Commit the regenerated `contrast-report.md` and re-project the skill with `sh .agents/skills/orcon-ui/scripts/project.sh`.
7. Never lower a minimum, and never mark a pair `decorative` to avoid a failure when it conveys information.
8. A visual change to an `Approved` component returns its owner review row to `Pending` ([review-protocol.md#owner-gate](review-protocol.md#owner-gate)).
