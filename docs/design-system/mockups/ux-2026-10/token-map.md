<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Token map: UX mockups (2026-10) to `@orcon/tokens`

Status: Applied (SPEC-019 UX1). Pending owner review with the rest of the baseline.

This map reconciles the CSS custom properties of the approved mockup (`index.html`: the `:root` block for the dark
theme and the `[data-theme="light"]` block for the light theme) with the tokens of `@orcon/tokens`. Rule R-UX-GEN-01
of SPEC-019 requires every mockup value to be an existing token or a new token added in both themes. Every color,
radius and type size the product screens use from the mockup is in the two tables below, with the same value in both
themes. The test `packages/js/tokens/test/mockup-token-map.test.ts` reads those two tables and fails when a row names
a token that does not exist or when a value differs.

A value the mockup needed and the tokens did not have was added: `surface.hover`, `border.strong`,
`brand.green-light`, `radius.base`, `radius.panel`, `font.size.2xs`, `font.size.display`, `size.form-width`, and the
light value of `mascot.panel`.

## Colors

`mockup` is the CSS variable of the mockup. `dark` and `light` are its values; `token` is the semantic token (or brand
token) that carries the same value in each theme.

| mockup | dark | light | token |
| --- | --- | --- | --- |
| --bg | #0B1120 | #F8FAFC | bg.app |
| --surface | #0F172A | #FFFFFF | surface.default |
| --raised | #1E293B | #FFFFFF | surface.raised |
| --sunken | #020617 | #F1F5F9 | surface.sunken |
| --hover | #162033 | #F1F5F9 | surface.hover |
| --fg | #F8FAFC | #0F172A | fg.default |
| --muted | #CBD5E1 | #475569 | fg.muted |
| --subtle | #94A3B8 | #64748B | fg.subtle |
| --disabled | #64748B | #94A3B8 | fg.disabled |
| --link | #22D3EE | #0E7490 | fg.link |
| --accent | #4ADE80 | #15803D | fg.accent |
| --primary | #4ADE80 | #15803D | action.primary.bg |
| --primary-h | #86EFAC | #166534 | action.primary.bg-hover |
| --primary-fg | #0F172A | #FFFFFF | action.primary.fg |
| --danger-btn | #F87171 | #B91C1C | action.danger.bg |
| --border | #1E293B | #E2E8F0 | border.default |
| --border-strong | #334155 | #CBD5E1 | border.strong |
| --control | #64748B | #64748B | border.control |
| --focus | #67E8F9 | #0E7490 | focus.ring |
| --n-fg | #CBD5E1 | #475569 | status.neutral.fg |
| --n-bg | #1E293B | #F1F5F9 | status.neutral.bg |
| --a-fg | #67E8F9 | #155E75 | status.active.fg |
| --a-bg | #083344 | #ECFEFF | status.active.bg |
| --s-fg | #86EFAC | #166534 | status.success.fg |
| --s-bg | #052E16 | #F0FDF4 | status.success.bg |
| --w-fg | #FCD34D | #92400E | status.warning.fg |
| --w-bg | #451A03 | #FFFBEB | status.warning.bg |
| --d-fg | #FCA5A5 | #B91C1C | status.danger.fg |
| --d-bg | #450A0A | #FEF2F2 | status.danger.bg |
| --u-fg | #C4B5FD | #6D28D9 | status.attention.fg |
| --u-bg | #2E1065 | #F5F3FF | status.attention.bg |
| --scrim (opacity .72 dark, .45 light) | #020617 | #0F172A | overlay.scrim |
| --cv-bg | #0B1120 | #F8FAFC | canvas.bg |
| --cv-grid | #1E293B | #CBD5E1 | canvas.grid |
| --cv-edge | #94A3B8 | #64748B | canvas.edge |
| --cv-node | #0F172A | #FFFFFF | canvas.node.bg |
| --cv-node-b | #334155 | #CBD5E1 | canvas.node.border |
| --cv-sel | #22C55E | #16A34A | canvas.selection |
| --cv-ok | #22C55E | #16A34A | canvas.edge-active |
| --st1-bg | #083344 | #ECFEFF | canvas.sticky.1.bg |
| --st1-b | #22D3EE | #0E7490 | canvas.sticky.1.border |
| --mascot-panel | #475569 | #E2E8F0 | mascot.panel |
| --brand-grad stop 1 (0%) | #84EB72 | #84EB72 | brand.green-light |
| --brand-grad stop 2 (55%) | #22C55E | #22C55E | brand.green |
| --brand-grad stop 3 (100%) | #06B6D4 | #06B6D4 | brand.cyan |

## Radius and type scale

`token` is a key of the scale tokens; `value` is the value the mockup uses.

| mockup | value | token |
| --- | --- | --- |
| kbd, tag radius | 4px | radius.sm |
| small button, menu item, organization tile | 6px | radius.base |
| button, input, chip row | 8px | radius.md |
| menu and popover | 10px | radius.panel |
| card | 12px | radius.lg |
| modal | 16px | radius.xl |
| canvas node tile | 18px | radius.2xl |
| pill, avatar, toggle | 9999px | radius.full |
| key caps, counts, section headings | 11px | font.size.2xs |
| helper text, chips | 12px | font.size.xs |
| menu items, labels | 13px | font.size.sm |
| body | 14px | font.size.base |
| section heading (h2) | 16px | font.size.md |
| dialog title | 20px | font.size.lg |
| page title | 24px | font.size.xl |
| sign-in card title | 28px | font.size.display |
| sign-in brand tagline | 32px | font.size.2xl |
| sign-in form card width | 380px | size.form-width |

## Decisions applied in UX1

1. **`--scrim`.** The scrim token stays opaque; the dialogs draw it with the opacity of the mockup (.72 dark, .45
   light), so the value of the table is the color and the opacity is applied where it is used.
2. **Shadows.** The elevation tokens keep their values. The small and medium shadows of the dark theme are the
   mockup's; the large shadow of the mockup adds a 1px ring that the product draws as the `border.strong` border of a
   floating panel. The light theme differs from the mockup by one step of alpha on the medium and large shadows,
   which is not visible; it is accepted.
3. **`--mascot-panel`.** The light theme uses the mockup value (`#E2E8F0`, `slate.200`) for the soft panel behind the
   mascot instead of `green.100`; the pair with `fg.default` still passes.
4. **`--cv-ok`.** It has the same values as the existing `canvas.edge-active`, so no new token was added.
5. **Gradients.** `--brand-grad` is allowed only on the logo and the marker of the current section (SPEC-019
   R-UX-GEN-02): `brand.green-light` completes its three stops and the `bg-brand-gradient` utility draws it. `--glow`
   is decorative and appears only on the brand panel of the sign-in page (`bg-brand-glow`, made of brand colors).
6. **Mockup-only values.** The large button text of 15px (the product uses 16px, `font.size.md`) and the 22px page
   title on a phone (the product keeps 24px, `font.size.xl`) are not used. The 5px corner of the project dot in the
   sidebar is drawn with `radius.base` (6px).

## Not mapped

- Literal colours inside component rules of the mockup (outside the variable blocks), including the coloured tiles
  for integrations (for example `.ni.ai`, `.ni.wa`). SPEC-019 R-UX-ICO replaces these tiles with the real brand icon
  of each connector, so they are not copied into the product. The project tiles of the sidebar take the success,
  active and neutral status colors in turn.
- Fonts. The mockup loads its fonts from an external font service. The product does not: `orcon-ui` foundations
  (typography, rule 2) require Inter and JetBrains Mono to be self-hosted. This is a property of the mockup file, not
  a token.
