<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Foundations

Status: Accepted | Version: 1.2.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. Visual values (colors, sizes, durations) live only in this file, [tokens.md](tokens.md), [canvas.md](canvas.md) and the generated [contrast-report.md](contrast-report.md). Code never uses these values directly: it uses the tokens that carry them ([tokens.md#usage-in-code](tokens.md#usage-in-code)).

## Principles

1. Professional and restrained. The interface is a work tool: calm surfaces, a navy and graphite base, and green-teal highlights reserved for execution, the primary action and selection.
2. Light surfaces are used for secondary layouts in the light theme; the dark theme keeps the navy character of the brand boards.
3. Status is never conveyed by color alone. Every status has an icon and a text label; color only reinforces them ([components.md#status](components.md#status)).
4. Familiar in interaction and canvas layout (style A, [canvas.md](canvas.md)), with Orcon's own palette, icons and brand. People who know n8n MUST find the canvas, node palette and node details view easy to use, and V1 MUST match that editor experience at least ([canvas.md#parity](canvas.md#parity)); Orcon MUST NOT copy n8n code, CSS, icons, assets, illustrations or branding.
5. No stock component-library look. Components derived from shadcn/ui MUST be re-themed (radius, color, shadow, focus, density) so that nothing looks like the library defaults.
6. Accessible by construction: WCAG 2.2 AA is the floor ([accessibility.md#target](accessibility.md#target)).
7. Motion is minimal and functional ([#motion](#motion)).

## Brand

- Name: **Orcon**. Descriptor: **ORCHESTRATION CONTROLLER**. Tagline: **Build. Orchestrate. Execute. Your way.**
- The two boards in `brand/reference/` (`orcon-board-primary.png`, `orcon-board-secondary.png`) are the visual source of truth. `brand/README.md` describes them and records their checksums.
- The primary board governs palette, mark, wordmark, mascot character and UI mood. The secondary board is reference only: its exploratory palette MUST NOT be used.
- The boards MUST NOT be edited, re-encoded, cropped or replaced. Derived exports go elsewhere under `brand/`.
- Reading of the primary board that the UI follows: navy blocks with a hexagonal open-loop symbol in a green-to-cyan gradient and a rounded white wordmark; a light panel (light slate-gray surfaces) with a navy wordmark; an app icon with a large radius (about 20 % of its side); a navy terminal with green and white monospaced text; curved green edges and side ports in the editor scene.

## Palette

The five exact brand colors (role `brand-exact`):

| Token | Value | Name on the board |
|---|---|---|
| `brand.green` | #22C55E | Bright green |
| `brand.green-dark` | #16A34A | Dark green |
| `brand.cyan` | #06B6D4 | Cyan |
| `brand.navy` | #0F172A | Midnight navy |
| `brand.slate` | #64748B | Slate |

Accessible brand-family shades, used for text, links and buttons:

| Theme | Green family | Cyan family |
|---|---|---|
| Light (darker shades) | green-700 #15803D, green-800 #166534 | cyan-700 #0E7490, cyan-800 #155E75 |
| Dark (lighter shades) | green-400 #4ADE80, green-300 #86EFAC | cyan-400 #22D3EE, cyan-300 #67E8F9 |

The ramps are aligned with the exact colors: green-500 = #22C55E, green-600 = #16A34A, cyan-500 = #06B6D4, slate-500 = #64748B, slate-900 = #0F172A, plus `navy-950` #0B1120 (darkest app background).

Functional status colors outside the brand palette (owner decision P2): **red**, **amber** and **violet**.

- Red = danger and failure.
- Amber = warning and succeeded with errors.
- Violet = attention. Its primary meaning is **UNCERTAIN**; within the same attention family it also marks suspended executions and requests awaiting a person.
- These three families MUST be used only to communicate status (badges, banners, alerts, node status, destructive actions). They MUST NOT be used decoratively, in illustrations, in charts unrelated to status, or as brand accents.

The exact shade of every status color is defined once in [tokens.md#semantic-colors](tokens.md#semantic-colors).

## Color usage

1. The three brand hues green #22C55E, green-dark #16A34A and cyan #06B6D4 MUST NEVER be used as a text color, and MUST NEVER be the background behind text (buttons, badges, chips, links, tags, banners).
2. They MAY be used for the logo, decorative fills, illustrations, borders, canvas lines, selection indicators and state fills in the dark theme. Where such a non-text use conveys information (an active edge, a selection outline, a status indicator), the pair MUST reach 3:1 against its background (WCAG 1.4.11).
3. Measured consequence: in the light theme #22C55E on the canvas background reaches only 2.18:1, so active edges and selection in light mode use the exact dark green #16A34A (3.15:1). In the dark theme they use #22C55E (8.26:1).
4. Exact navy and exact slate are part of the neutral ramp (owner decision P1). Navy #0F172A is the primary text color in the light theme (17.85:1 on white) and the panel surface in the dark theme, with text on it (17.06:1). The restriction of rule 1 applies to the green and cyan hues, not to navy.
5. Slate #64748B is tertiary text in the light theme, only on `bg.app`, `surface.default` and `surface.raised` (4.55 to 4.76:1); it is the control border in both themes.
6. Text, links and buttons use the accessible brand-family shades of [#palette](#palette): darker shades in the light theme, lighter shades in the dark theme.
7. Green-teal accents are reserved for execution (running paths, active edges), the primary action, focus and selection. They MUST NOT be spread across unrelated decoration.
8. Red, amber and violet follow [#palette](#palette): status only.
9. Every foreground and background combination that a component uses MUST be a declared, certified pair ([tokens.md#contrast-pairs](tokens.md#contrast-pairs)).

## Themes

1. Two complete themes: light and dark. Both are first-class; the dark theme is designed separately, not inverted.
2. Default: follow the operating system (`prefers-color-scheme`). The user MAY choose System, Light or Dark in Settings › Appearance.
3. `:root` carries the light theme. Dark applies through `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { … } }` and through `[data-theme="dark"]`. The default needs no JavaScript.
4. A user preference sets `data-theme` on `<html>`. It is applied before first paint by a small external script (`theme-init.js`), never by an inline script (Content Security Policy). The CSS `color-scheme` property MUST match the active theme.
5. Until a preferences API exists, theme, density and locale preferences are stored in `localStorage`; every read and write is wrapped in `try/catch` and the UI works when storage is unavailable.
6. Dark theme design: surfaces are stepped by lightness (app background darkest, panels on navy, raised surfaces lighter); shadows are replaced by a border plus the raised surface; accents are lighter shades.
7. Every component, story and screen template MUST be correct in both themes.

## Typography

1. UI font: **Inter** (variable). Monospace: **JetBrains Mono** (variable) for code, expressions, identifiers, JSON and timestamps.
2. Both fonts are self-hosted from `@fontsource-variable/inter` and `@fontsource-variable/jetbrains-mono` (OFL-1.1), bundled as `.woff2` and embedded in the binary. Requests to external font services (Google Fonts or any other domain) are forbidden. The OFL license texts are included in the third-party notices.
3. Type scale (size/line-height in px), identical in both densities:

| Step | Size / line-height | Typical use |
|---|---|---|
| xs | 12/16 | Captions, badges, edge chips |
| sm | 13/18 | Secondary text, table meta |
| base | 14/20 | Body text, controls, table cells |
| md | 16/24 | Prose, dialog body |
| lg | 20/28 | Section titles |
| xl | 24/32 | Page titles |
| 2xl | 32/40 | Welcome and empty-state headlines |
| mono | 13/20 | Code, identifiers, JSON, timestamps |

4. Weights: 400 (regular), 500 (medium), 600 (semibold), 700 (bold). No other weights.
5. Sentence case for all UI text ([content-and-i18n.md#voice](content-and-i18n.md#voice)).
6. The wordmark is an SVG asset ([#brand-mark](#brand-mark)). It MUST NEVER be typeset with a font.

## Iconography

1. Icon library: **Lucide** (`lucide-react`, ISC). Sizes 16, 20 and 24 px; stroke width 2 (1.5 at 24 px).
2. Custom icons: 24 px grid, stroke 2, round caps and joins, no fills unless the Lucide equivalent uses one. They live in `packages/js/ui/src/icons/`, each with a story, and pass the owner review.
3. Icons that act as the only content of a control (IconButton) MUST have an accessible label from the message catalog. Decorative icons next to text are `aria-hidden`.
4. `ConnectorIcon` is the only way to show a connector logo:
   - Orcon frame: 32 px (comfortable) / 24 px (compact), radius 8, `surface.sunken` fill and `border.default` border.
   - The connector SVG is fitted into a safe area of 20 px (comfortable) / 16 px (compact), centered.
   - Rendered with `<img>`, never as inline SVG, so catalog content never executes in the page.
   - `alt` is the connector display name.
   - Catalog admission: square SVG, no scripts, no external references, at most 8 KiB.
5. Third-party logos are used only to identify the corresponding service, inside `ConnectorIcon`. They MUST NOT be recolored, combined with the Orcon mark, or used decoratively.
6. n8n icons and node artwork MUST NOT be used or imitated. Inside canvas nodes a connector logo is shown through `ConnectorIcon variant="tile"` (the bare `<img>` in the 40 px icon box, no inner frame); built-in nodes use Lucide icons ([canvas.md#node-tile](canvas.md#node-tile)).

## Mascot

1. Allowed contexts only: the sign-in screen and welcome/onboarding (pose "welcome", the crossed-arms cut-out), first-run empty states where nothing exists yet (pose "happy"), successful n8n import completion (pose "happy"), full-page error screens — the not-found page and the generic error page — (pose "thinking"), and documentation.
2. Forbidden contexts: inline errors (ProblemAlert), real execution errors, security warnings and approvals, toasts, help, the canvas, and dense views (tables, execution detail, the attention inbox). The lint rule of [SKILL.md](../SKILL.md) "Mascot and connector icon boundaries" enforces this.
3. The mascot is lazy-loaded and decorative: `alt=""` and `aria-hidden="true"`. The adjacent text carries all meaning.
4. Maximum height inside the app: 160 px.
5. `Mascot` renders the approved cut-outs of the primary board listed in `brand/manifest.json` (`welcome` = crossed arms, `happy` = laughing expression, `thinking` = thinking expression). The cut-outs are never redrawn, simplified or recolored; a new pose comes only from a new approved cut-out.
6. The mascot sits on a soft rounded panel in a token color in both themes, like the panel behind the crossed-arms pose on the primary board.

## Brand mark

1. `BrandMark` renders the approved production lockups from `brand/logo/` (`orcon-lockup-on-dark.svg` in the dark theme, `orcon-lockup-on-light.svg` in the light theme), chosen by CSS from the active theme. `BrandHero` renders the lockup with the descriptor and the tagline on the sign-in screen.
2. Every brand SVG is a true vector: no `<image>`, no `data:` URI, no base64, no external reference and no `<text>`; `make brand-check` enforces it.
3. The symbol MUST NEVER be redrawn, approximated or substituted with a generic hexagon.
4. Replacing the placeholder with the final asset keeps the component contract (props, sizes, accessible name) unchanged.
5. Accessible name: "Orcon" (from the catalog) when the mark is the only identification; `aria-hidden` when adjacent text already says "Orcon".

## Layout

1. Breakpoints (min-width): sm 640, md 768, lg 1024, xl 1280, 2xl 1536 px.
2. App shell:
   - ≥ lg: persistent sidebar navigation.
   - md to lg: collapsible rail.
   - < md: bottom navigation plus a drawer for the remaining destinations.
3. Maximum content width for forms and settings: 1280 px. Lists, tables and the canvas MAY use the full width.
4. Spacing follows the 4 px scale of [tokens.md#scales](tokens.md#scales); no other spacing values.
5. Responsive behavior (owner decision Q10):
   - Phone (390 px reference width) is fully operable for the attention and approvals inbox, the execution list and execution detail (continue, retry, cancel), notifications, enabling and disabling a flow, and appearance settings.
   - Below md (768 px) the canvas is read-only ([canvas.md#mobile](canvas.md#mobile)). Flow editing requires md or wider (tablet or desktop).
   - Every layout tolerates text 30 % longer than English ([content-and-i18n.md#expansion](content-and-i18n.md#expansion)).
6. Radius: generous on containers (12 to 16 px: cards, panels, dialogs, nodes) and moderate on controls (8 px: buttons, inputs, menus).

## Density

1. Two densities, set by `data-density` on `<html>`: **comfortable** (default) and **compact** (per-user preference).

| Measure | Comfortable | Compact |
|---|---|---|
| Table row height | 40 | 32 |
| Control height | 36 | 28 |
| Control horizontal padding | 12 | 8 |
| Gap between related controls | 12 | 8 |

2. Typography does not change with density.
3. Touch targets: with `@media (pointer: coarse)`, every interactive control has a hit area of at least 44×44 px in both densities. The visual size MAY stay smaller; the hit area grows. The only exception is a link inside running prose (container marked `data-prose`), per the inline exception of WCAG 2.5.8.
4. With a fine pointer, every target is at least 24×24 px ([accessibility.md#targets](accessibility.md#targets)).
5. Every component and screen MUST be correct in both densities.

## Elevation

1. Light theme: shadows sm, md and lg express elevation (sm: cards and nodes; md: popovers and menus; lg: dialogs and sheets).
2. Dark theme: elevation is expressed by a lighter raised surface plus a border; shadows are faint or absent.
3. Z layers: base 0, sticky 10, dropdown 20, overlay 30, modal 40, toast 50, tooltip 60. No other z-index values.
4. Sticky UI MUST NOT hide the focused element ([accessibility.md#focus](accessibility.md#focus)).

## Motion

1. Durations: `duration.fast` 120 ms, `duration.base` 160 ms, `duration.slow` 200 ms. Nothing longer.
2. Easing: `easing.standard` cubic-bezier(0.2, 0, 0, 1); `easing.enter` cubic-bezier(0, 0, 0.2, 1); `easing.exit` cubic-bezier(0.4, 0, 1, 1).
3. Allowed uses only: a state change, the spatial origin of a panel or sheet, and feedback to a click or press.
4. Motion MUST NEVER block interaction and MUST NEVER animate large graphs continuously. The only looping motion on the canvas is on running elements (run ring, running spinner, running edge), and it is off with reduced motion ([canvas.md#execution-states](canvas.md#execution-states)).
5. With `prefers-reduced-motion: reduce`, all durations are 0 ms and the running indicator does not spin; status stays visible by icon and text.
6. No decorative, looping or parallax animation. Functional running indicators are the only loops, with durations `motion.loop.*` ([tokens.md#canvas-tokens](tokens.md#canvas-tokens)).
