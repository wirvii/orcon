---
name: orcon-ui
description: Builds and reviews Orcon web UI (apps/web, packages/js) with the Orcon design system - tokens, themed shadcn/Radix components, canvas visual language, i18n, WCAG 2.2 AA and the automated UI checks. Use when creating or changing any UI component, screen, style, story, icon, user-visible string or canvas element, or when reviewing a UI pull request.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with pnpm 9 and Node 22.
metadata:
  version: 1.0.0
  pinned: true
  owner: wirvii
---

# Orcon UI

Step-and-check list for building Orcon UI. The rules live in `references/`; this file tells you which apply and how to prove you followed them. Read the referenced sections before you write code; never work from memory.

The rules are `Proposed — pending owner approval` until the owner approves every base component and screen template ([owner gate](references/review-protocol.md#owner-gate)).

## When To Use

- Creating or changing anything under `apps/web` or `packages/js/{tokens,ui,lint-config}`.
- Adding or editing a story, icon, user-visible string, style, token or canvas element.
- Adding a screen template or building a product screen on top of one.
- Reviewing a UI pull request.

Reference map (load only what the task needs):

| Reference | Load when |
|---|---|
| [foundations.md](references/foundations.md) | Color, theme, type, icons, mascot, brand mark, layout, density, elevation, motion |
| [tokens.md](references/tokens.md) | Using, naming or adding tokens; contrast pairs |
| [components.md](references/components.md) | Building or changing any component; status, errors, notifications, redaction, screen templates |
| [canvas.md](references/canvas.md) | Anything on the workflow canvas |
| [content-and-i18n.md](references/content-and-i18n.md) | Any user-visible text, formatting or translation |
| [accessibility.md](references/accessibility.md) | Every change (WCAG 2.2 AA), and the manual pass |
| [contrast-report.md](references/contrast-report.md) | Checking a certified pair (generated file) |
| [component-checklist.md](references/component-checklist.md) | Every component change |
| [screen-checklist.md](references/screen-checklist.md) | Every screen or template change |
| [review-protocol.md](references/review-protocol.md) | Screenshot review and the owner gate |

## Critical Rules

1. Tokens only: no hex, rgb or hsl literals, no arbitrary values, no Tailwind default palette or scale, no inline styles outside canvas positioning ([tokens](references/tokens.md#usage-in-code)).
2. Exact green and cyan brand hues are never text and never behind text; use the brand-family shades for text, links and buttons ([color usage](references/foundations.md#color-usage)).
3. Red, amber and violet are only for status, never decorative; violet means attention, primarily UNCERTAIN ([palette](references/foundations.md#palette)).
4. Status is always icon + text; color only reinforces it ([status](references/components.md#status)).
5. UNCERTAIN is never shown or named as failure or success, and "repeat" is never the primary action for it ([status](references/components.md#status)).
6. Every component declares its states and its fg/bg pairs in `meta.ts`; every pair is certified in `@orcon/tokens` ([contract](references/components.md#contract), [contrast pairs](references/tokens.md#contrast-pairs)).
7. A missing value becomes a token in `@orcon/tokens` for both themes, with declared pairs; never a value in a component ([adding a token](references/tokens.md#adding-a-token)).
8. Every user-visible string, including `aria-label`, `title`, `placeholder` and `alt`, comes from the en and es catalogs through `react-intl` ([strings](references/content-and-i18n.md#strings)).
9. ICU plural and select only; never concatenate translated fragments ([ICU](references/content-and-i18n.md#icu)).
10. Dates, times, numbers and durations are formatted only with Intl ([formatting](references/content-and-i18n.md#formatting)).
11. Layouts tolerate 30 % longer text; truncation only with `data-truncate` plus a tooltip ([expansion](references/content-and-i18n.md#expansion)).
12. Light and dark themes are both complete; nothing ships in one theme only ([themes](references/foundations.md#themes)).
13. Comfortable and compact densities are both correct; heights come from density tokens ([density](references/foundations.md#density)).
14. Interactive targets are at least 44×44 px on coarse pointers and 24×24 px otherwise ([targets](references/accessibility.md#targets)).
15. Visible focus ring on every interactive element, never obscured by sticky UI; full keyboard operation ([focus](references/accessibility.md#focus), [keyboard](references/accessibility.md#keyboard)).
16. Motion is 120 to 200 ms, functional only, and off with reduced motion ([motion](references/foundations.md#motion)).
17. Mascot only in welcome, empty states, n8n import success, 404 and docs; never on errors, security warnings, canvas or dense views; neutral placeholder until approved assets exist ([mascot](references/foundations.md#mascot)).
18. Brand mark is a neutral placeholder until approved assets exist; never redraw the symbol ([brand mark](references/foundations.md#brand-mark)).
19. Connector logos only through `ConnectorIcon` (`<img>`, never inline SVG) ([iconography](references/foundations.md#iconography)).
20. Canvas is read-only below md; editing needs tablet or desktop ([canvas mobile](references/canvas.md#mobile)).
21. Every canvas pointer action has a keyboard path, and `CanvasOutline` describes the graph to screen readers ([canvas keyboard](references/canvas.md#keyboard), [screen reader](references/canvas.md#screen-reader)).
22. Server errors (`application/problem+json`) render through `ProblemAlert` and `FormField`; input is never cleared on error ([forms and errors](references/components.md#forms-and-errors)).
23. Protected data without permission renders as `RedactedValue`, never empty; copy never copies hidden values ([redaction](references/components.md#redaction)).
24. Decisions never live in toasts; live announcements are polite and coalesced ([notifications](references/components.md#notifications), [live regions](references/accessibility.md#live-regions)).
25. No n8n code, icons, artwork or branding; familiar interaction, original look ([principles](references/foundations.md#principles)).
26. Code comments, stories, commit messages and PR text never name coding-agent products and never carry AI authorship trailers; use neutral contributor wording.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| Token lint (TS/TSX) | No hex/rgb/hsl literals in code; no Tailwind arbitrary classes (`[…]`); no default palette or scale utilities; spacing only on the scale; inline `style` only for canvas positions with a reason comment | Use or create a token in `@orcon/tokens` |
| Token lint (CSS) | No hex, color or size values outside `packages/js/tokens`; color, space and radius only through `var(--orcon-*)` | Move the value to a token |
| Hard-coded strings | No literal JSX text or visible attribute (`aria-label`, `title`, `placeholder`, `alt`, `label`); valid ICU messages; en and es have the same keys and no unused keys | Use `FormattedMessage` or `intl.formatMessage` and add the key to en and es |
| Mascot and connector icon boundaries | `Mascot` is not imported in canvas, execution, error, security or dense views; `ConnectorIcon` is the only path for connector SVGs | Remove the import or use `EmptyState` in an allowed context |
| Contrast | Every fg/bg pair × theme meets 4.5:1 (text) or 3:1 (UI); every color token is in a pair or marked decorative; no text pair uses an exact brand hue; every pair a component uses is declared; the report is current | Adjust the semantic token or declare the correct pair; never lower a minimum |
| Accessibility (axe) | Zero axe violations (WCAG A/AA 2.0 to 2.2) in every story in light and dark, every template and the `apps/web` screen | Fix semantics, accessible names, roles or rendered contrast |
| Story coverage | Every component has `meta.ts` with `states`, one story per state and `Matrix`; the visual configuration covers theme × viewport × density | Add the missing stories or states |
| Visual regression | Every `Matrix` and template × {light, dark} × {mobile, desktop} × {comfortable, compact} (plus tablet for S3) matches its baseline | If intended, regenerate in the pinned container and ask for re-approval of approved items |
| Text expansion | In the pseudo-locale `xx-LS`, no text container overflows unless truncation is declared (`data-truncate` + tooltip) | Allow wrapping or declare truncation |
| Targets and density | With touch and a coarse pointer, every interactive control is at least 44×44; table rows are 40 px (comfortable) and 32 px (compact) | Use `h-control` / `h-row` and the base component hit area |
| Fonts self-hosted | The web build references no external font domain and contains `.woff2` files | Import fonts from `@fontsource-variable/*` |
| Skill conformance | Agent Skills standard, mneme template and a byte-identical projection | Fix `SKILL.md` or its frontmatter, or re-project with `scripts/project.sh` |

## Verification

1. `sh .agents/skills/orcon-ui/scripts/verify.sh` exits 0 (lint, typecheck, tokens test, i18n check, unit, stories, accessibility, targets and visual tests).
2. The screenshot review findings table is attached to the pull request ([review protocol](references/review-protocol.md#findings-table)).
3. The manual keyboard and screen-reader pass is recorded in the pull request ([manual pass](references/accessibility.md#manual-pass)).
4. `sh .agents/skills/orcon-ui/validation/run.sh` exits 0 and `.claude/skills/orcon-ui` is identical to `.agents/skills/orcon-ui` after `sh .agents/skills/orcon-ui/scripts/project.sh`.
5. When the skill itself changes: `sh .agents/skills/orcon-ui/scripts/project.sh --home && mneme skills lint orcon-ui && mneme skills validate orcon-ui` pass.

## Workflow

Follow these steps for every component or screen. The detailed checklists are [component-checklist.md](references/component-checklist.md) and [screen-checklist.md](references/screen-checklist.md); copy the one that applies into the pull request.

1. Load this skill and read the `references/` sections that apply ([contract](references/components.md#contract)).
2. Look for an existing component or pattern in `@orcon/ui` (Storybook, `src/components`); reuse before creating ([inventory](references/components.md#inventory)).
3. Declare in `<Name>.meta.ts` the states (`default`, `hover`, `focus-visible`, `active`, `disabled`, `loading`, `error`, `empty` as they apply, plus domain states) and the fg/bg pairs used ([states](references/components.md#states)).
4. If a token is missing, add it in `@orcon/tokens` for both themes, declare its pairs in `pairs.json` and run the contrast test; never a value in the component ([adding a token](references/tokens.md#adding-a-token)).
5. Implement with token utilities, Radix primitives, Lucide icons and `react-intl` strings ([usage in code](references/tokens.md#usage-in-code)).
6. Write one story per state plus `Matrix`; check light and dark, comfortable and compact, mobile and desktop, and the pseudo-locale ([expansion](references/content-and-i18n.md#expansion)).
7. Run `sh .agents/skills/orcon-ui/scripts/verify.sh`.
8. Review screenshots against `brand/reference/*.png` with the review protocol and attach the findings table to the pull request ([compare](references/review-protocol.md#compare)).
9. Do the manual keyboard and screen-reader pass and record it in the pull request ([manual pass](references/accessibility.md#manual-pass)).
10. For a base component or primary screen, add a `Pending` row to "Owner review status" in `docs/design-system/README.md` using `assets/review-record-template.md` ([owner gate](references/review-protocol.md#owner-gate)).
