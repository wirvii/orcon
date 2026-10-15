<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Token map: UX mockups (2026-10) to `@orcon/tokens`

Status: Pending owner review (SPEC-019 UX0)

This table reconciles the CSS custom properties of the approved mockup (`index.html`, the `:root` block for the dark theme and the `[data-theme="light"]` block for the light theme) with the semantic tokens of `@orcon/tokens` (`packages/js/tokens/src/semantic/dark.json` and `light.json`). Rule R-UX-GEN-01 of SPEC-019 requires every mockup value to map to an existing semantic token or to a new token added in both themes.

Values in the table are the resolved hexadecimal values of the tokens at the time this map was written. Each row is one of:

- **Match**: the token exists and its value is equal to the mockup value in both themes.
- **Differs**: the token exists, but its value is not equal to the mockup value in at least one theme. A decision is needed (see the notes).
- **No token**: no semantic token carries this value. A new token is needed, or the mockup value is decorative and stays out of the product.

The map is a snapshot. The test `packages/js/tokens/test/mockup-token-map.test.ts` (SPEC-019 UX1) verifies that every row points to a token that exists and that the value matches in both themes.

## Surfaces, text and actions

| Mockup variable | Dark | Light | Token | Result |
|---|---|---|---|---|
| `--bg` | `#0B1120` | `#F8FAFC` | `bg.app` | Match |
| `--surface` | `#0F172A` | `#FFFFFF` | `surface.default` | Match |
| `--raised` | `#1E293B` | `#FFFFFF` | `surface.raised` | Match |
| `--sunken` | `#020617` | `#F1F5F9` | `surface.sunken` | Match |
| `--hover` | `#162033` | `#F1F5F9` | none | No token |
| `--fg` | `#F8FAFC` | `#0F172A` | `fg.default` | Match |
| `--muted` | `#CBD5E1` | `#475569` | `fg.muted` | Match |
| `--subtle` | `#94A3B8` | `#64748B` | `fg.subtle` | Match |
| `--disabled` | `#64748B` | `#94A3B8` | `fg.disabled` | Match |
| `--link` | `#22D3EE` | `#0E7490` | `fg.link` | Match |
| `--accent` | `#4ADE80` | `#15803D` | `fg.accent` | Match |
| `--primary` | `#4ADE80` | `#15803D` | `action.primary.bg` | Match |
| `--primary-h` | `#86EFAC` | `#166534` | `action.primary.bg-hover` | Match |
| `--primary-fg` | `#0F172A` | `#FFFFFF` | `action.primary.fg` | Match |
| `--danger-btn` | `#F87171` | `#B91C1C` | `action.danger.bg` | Match |
| `--border` | `#1E293B` | `#E2E8F0` | `border.default` | Match |
| `--border-strong` | `#334155` | `#CBD5E1` | none | No token |
| `--control` | `#64748B` | `#64748B` | `border.control` | Match |
| `--focus` | `#67E8F9` | `#0E7490` | `focus.ring` | Match |

## Status families (SPEC-019 R-UX-GEN-03)

| Mockup variable | Dark | Light | Token | Result |
|---|---|---|---|---|
| `--n-fg` | `#CBD5E1` | `#475569` | `status.neutral.fg` | Match |
| `--n-bg` | `#1E293B` | `#F1F5F9` | `status.neutral.bg` | Match |
| `--a-fg` | `#67E8F9` | `#155E75` | `status.active.fg` | Match |
| `--a-bg` | `#083344` | `#ECFEFF` | `status.active.bg` | Match |
| `--s-fg` | `#86EFAC` | `#166534` | `status.success.fg` | Match |
| `--s-bg` | `#052E16` | `#F0FDF4` | `status.success.bg` | Match |
| `--w-fg` | `#FCD34D` | `#92400E` | `status.warning.fg` | Match |
| `--w-bg` | `#451A03` | `#FFFBEB` | `status.warning.bg` | Match |
| `--d-fg` | `#FCA5A5` | `#B91C1C` | `status.danger.fg` | Match |
| `--d-bg` | `#450A0A` | `#FEF2F2` | `status.danger.bg` | Match |
| `--u-fg` | `#C4B5FD` | `#6D28D9` | `status.attention.fg` | Match |
| `--u-bg` | `#2E1065` | `#F5F3FF` | `status.attention.bg` | Match |

The `--u-*` family is the attention family. SPEC-019 uses it for the state "Para revisar" (Needs review). The family is unchanged; only the label changes (orcon-ui `components.md` status map, version 1.3.0).

## Overlay, elevation and decoration

| Mockup variable | Dark | Light | Token | Result |
|---|---|---|---|---|
| `--scrim` | `rgba(2,6,23,.72)` | `rgba(15,23,42,.45)` | `overlay.scrim` (`#020617` and `#0F172A`, opaque) | Differs |
| `--sh-sm` | `0 1px 2px #0206174D` | `0 1px 2px #0F172A14` | `elevation.sm` | Differs |
| `--sh-md` | `0 4px 8px -2px #02061759` | `0 4px 8px -2px #0F172A1A, 0 2px 4px -2px #0F172A0F` | `elevation.md` | Differs |
| `--sh-lg` | `0 12px 24px -4px #02061766, 0 0 0 1px #1E293B` | `0 12px 24px -4px #0F172A24, 0 0 0 1px #E2E8F0` | `elevation.lg` | Differs |
| `--glow` | radial gradient, green 13% alpha | radial gradient, green 8% alpha | none | No token (decorative) |
| `--brand-grad` | linear gradient, brand green to cyan | not defined | none | No token (brand) |

## Canvas and mascot

| Mockup variable | Dark | Light | Token | Result |
|---|---|---|---|---|
| `--cv-bg` | `#0B1120` | `#F8FAFC` | `canvas.bg` | Match |
| `--cv-grid` | `#1E293B` | `#CBD5E1` | `canvas.grid` | Match |
| `--cv-edge` | `#94A3B8` | `#64748B` | `canvas.edge` | Match |
| `--cv-node` | `#0F172A` | `#FFFFFF` | `canvas.node.bg` | Match |
| `--cv-node-b` | `#334155` | `#CBD5E1` | `canvas.node.border` | Match |
| `--cv-sel` | `#22C55E` | `#16A34A` | `canvas.selection` | Match |
| `--cv-ok` | `#22C55E` | `#16A34A` | none | No token |
| `--st1-bg` | `#083344` | `#ECFEFF` | `canvas.sticky.1.bg` | Match |
| `--st1-b` | `#22D3EE` | `#0E7490` | `canvas.sticky.1.border` | Match |
| `--mascot-panel` | `#475569` | `#E2E8F0` | `mascot.panel` (`#475569` and `#DCFCE7`) | Differs (light) |

## Decisions for UX1

1. **`--scrim`.** The mockup uses a translucent scrim; `overlay.scrim` is opaque. UX1 either adds a translucent scrim token in both themes or changes `overlay.scrim` after review. Either way the change is listed in the PR.
2. **Shadows.** The three `--sh-*` values are not the values of `elevation.*`. UX1 reconciles the shadow primitives (`packages/js/tokens/src/primitives/shadow.json`) so that both themes match the mockup, or records the difference as approved by the owner.
3. **`--mascot-panel` in the light theme.** The mockup uses `#E2E8F0`; the token uses `green.100`. The owner decides which one the light theme uses. orcon-ui `foundations.md` section Mascot, point 6, requires a soft panel in a token colour in both themes.
4. **No-token values.** `--hover`, `--border-strong` and `--cv-ok` are new semantic tokens, added in both themes with their pairs (orcon-ui rules 6 and 7). Names are proposed as `surface.hover`, `border.strong` and `canvas.ok`.
5. **Gradients.** `--glow` is decorative and stays out of the product unless the owner approves it. `--brand-grad` is allowed only on the logo and the active section indicator (SPEC-019 R-UX-GEN-02); UX1 adds a brand gradient token only when the logo component needs it.

## Not mapped

- Literal colours inside component rules of the mockup (outside the variable blocks), including the coloured tiles for integrations (for example `.ni.ai`, `.ni.wa`). SPEC-019 R-UX-ICO replaces these tiles with the real brand icon of each connector, so they are not copied into the product. UX1 maps any literal colour it keeps to a token or records why it is kept.
- Fonts. The mockup loads its fonts from an external font service. The product does not: `orcon-ui` foundations (typography, rule 2) require Inter and JetBrains Mono to be self-hosted. This difference is a property of the mockup file, not a token.
