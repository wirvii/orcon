<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Component checklist

Status: Accepted | Version: 1.3.0 | Source: SPEC-007

Step-by-step procedure for every new or changed component in `@orcon/ui` (including canvas primitives). Copy the checklist into the pull request and tick each item. Each step links to the rule that governs it.

## Steps

- [ ] 1. Load the `orcon-ui` skill and read the reference sections that apply to this component. Do not work from memory ([components.md#contract](components.md#contract), [foundations.md#principles](foundations.md#principles)).
- [ ] 2. Search `@orcon/ui` (Storybook and `src/components`) for an existing component or pattern; reuse or extend it before creating a new one ([components.md#inventory](components.md#inventory)).
- [ ] 3. Declare in `<Name>.meta.ts` the `states` (`default`, `hover`, `focus-visible`, `active`, `disabled`, `loading`, `error`, `empty` as they apply, plus domain states) and the fg/bg `pairs` the component renders ([components.md#states](components.md#states), [tokens.md#contrast-pairs](tokens.md#contrast-pairs)).
- [ ] 4. If a token is missing, add it in `@orcon/tokens` for both themes (or both densities), declare its pairs in `pairs.json` and run the contrast test. Never put a value in the component ([tokens.md#adding-a-token](tokens.md#adding-a-token)).
- [ ] 5. Implement with token utilities, Radix primitives, Lucide icons and `react-intl` strings ([tokens.md#usage-in-code](tokens.md#usage-in-code), [foundations.md#iconography](foundations.md#iconography), [content-and-i18n.md#strings](content-and-i18n.md#strings)).
- [ ] 6. Write one story per state plus `Matrix`; check light and dark, comfortable and compact, mobile and desktop, and the pseudo-locale `xx-LS` ([components.md#contract](components.md#contract), [content-and-i18n.md#expansion](content-and-i18n.md#expansion)).
- [ ] 7. Run `sh .agents/skills/orcon-ui/scripts/verify.sh` until it exits 0.
- [ ] 8. Review the screenshots against `brand/reference/*.png` following the review protocol and attach the findings table to the pull request ([review-protocol.md#compare](review-protocol.md#compare), [review-protocol.md#findings-table](review-protocol.md#findings-table)).
- [ ] 9. Do the manual keyboard and screen-reader pass and record it in the pull request ([accessibility.md#manual-pass](accessibility.md#manual-pass)).
- [ ] 10. For a base component, add or reset its row to `Pending` in "Owner review status" of `docs/design-system/README.md`, using `assets/review-record-template.md` ([review-protocol.md#owner-gate](review-protocol.md#owner-gate)).

## Rule checks

- [ ] No hex, arbitrary values, default Tailwind palette or inline styles ([tokens.md#usage-in-code](tokens.md#usage-in-code)).
- [ ] Exact green and cyan brand hues are not used as text or behind text ([foundations.md#color-usage](foundations.md#color-usage)).
- [ ] Red, amber and violet appear only for status ([foundations.md#palette](foundations.md#palette)).
- [ ] Status is icon + text; UNCERTAIN rules respected ([components.md#status](components.md#status)).
- [ ] Visible focus ring, keyboard operable, accessible name ([accessibility.md#focus](accessibility.md#focus), [accessibility.md#keyboard](accessibility.md#keyboard)).
- [ ] Targets ≥ 44 px on coarse pointers ([accessibility.md#targets](accessibility.md#targets)).
- [ ] Motion only 120 to 200 ms, functional, off with reduced motion ([foundations.md#motion](foundations.md#motion)).
- [ ] Mascot only in allowed contexts (sign-in and welcome, first-run empty states, full-page error screens; never inline errors, security warnings, toasts, help, canvas or dense views); connector logos only through `ConnectorIcon` ([foundations.md#mascot](foundations.md#mascot), [foundations.md#iconography](foundations.md#iconography)).
- [ ] Errors through `ProblemAlert` / `FormField`; protected data through `RedactedValue` ([components.md#forms-and-errors](components.md#forms-and-errors), [components.md#redaction](components.md#redaction)).
- [ ] No n8n code, icons or branding ([foundations.md#principles](foundations.md#principles)).
- [ ] Comments, stories, commit messages and PR text use neutral contributor wording (no coding-agent product names, no AI authorship trailers).
