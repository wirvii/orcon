# Review protocol

Status: Proposed — pending owner approval | Version: 1.0 | Source: SPEC-007

How a UI change is reviewed visually before the owner sees it, and how the owner gate works. The rules being checked live in [foundations.md](foundations.md), [components.md](components.md) and [canvas.md](canvas.md); this document only describes the procedure.

## Capture

1. Build the static Storybook from the commit under review: `pnpm --filter @orcon/ui build-storybook`.
2. Capture every affected `Matrix` story and screen template in the 8 combinations: {light, dark} × {mobile 390×844, desktop 1440×900} × {comfortable, compact}. The flow editor template (S3) adds tablet 1024×768.
3. Use the same pinned Playwright container as the visual regression suite (`pnpm --filter @orcon/ui test:visual:docker`), with reduced motion enabled, so captures are reproducible.
4. Also capture the pseudo-locale `xx-LS` for screens with long text.
5. Name captures `<story-id>--<theme>--<viewport>--<density>.png` and attach them to the pull request (or link the CI artifact).

## Compare

Place each capture side by side with `brand/reference/orcon-board-primary.png` (governing) and, for character and poses only, `brand/reference/orcon-board-secondary.png`. Check each criterion:

| Criterion | Pass when |
|---|---|
| Palette | Only token colors appear; navy/graphite base; light surfaces in light mode; matches the primary board, not the secondary board's exploratory palette |
| Radii | Containers 12 to 16 px, controls 8 px; consistent with the board's rounded character |
| Character | Professional, calm, restrained; reads as Orcon, consistent with the boards' mood |
| Restraint | No decorative gradients, illustrations or motion beyond what the rules allow |
| Accent use | Green-teal appears only for execution, primary action, focus and selection |
| Status colors | Red, amber and violet appear only on status elements, always with icon + text |
| No stock look | Nothing looks like default shadcn/ui (radius, gray palette, shadows, focus ring) |
| No n8n traits | No n8n colors, node shapes, icons, artwork or layouts copied |
| Brand assets | BrandMark and Mascot show the neutral placeholders until approved assets exist; nothing tries to redraw the symbol or the character |
| Themes and densities | Dark is designed (stepped surfaces, lighter accents), not inverted; compact keeps hierarchy and readability |

## Findings table

Attach this table to the pull request, one row per finding (or a single row "No findings" per capture set):

```
| Capture | Criterion | Finding | Severity (blocker / major / minor) | Action |
|---|---|---|---|---|
| button-matrix--dark--desktop--compact | Accent use | Secondary button uses green border | major | Use border.control |
```

1. The author's review is followed by an independent review pass by a different contributor, who adds their own rows.
2. Blockers and majors are fixed before the owner review; minors may be listed as follow-ups with a backlog reference.

## Owner gate

1. **Record**: "Owner review status" in `docs/design-system/README.md` has one row per base component (the inventory in [components.md#inventory](components.md#inventory)) and per screen template S1 to S9. New or reset rows use `assets/review-record-template.md` with status `Pending`.
2. **Owner review**: the owner reviews in the static Storybook of the commit named in the row and sets each row to `Approved` or `Changes requested`, with date and notes. Only the owner sets `Approved`.
3. `BrandMark` and `Mascot` are reviewed as neutral placeholders; their final versions are reviewed when the approved brand assets are delivered.
4. **Acceptance**: when every row is `Approved`, every normative reference changes its status line to `Status: Accepted` and `metadata.version` in `SKILL.md` becomes `1.1.0`. Until then, product screens must not rely on the design system as an accepted source.
5. **Re-approval**: changing the visual baseline of an `Approved` component or template returns its row to `Pending`.
