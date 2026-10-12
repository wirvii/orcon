# Screen checklist

Status: Proposed — pending owner approval | Version: 1.0 | Source: SPEC-007

Step-by-step procedure for every screen template in `@orcon/ui` (`src/screens/`) and every screen in `apps/web`. Copy the checklist into the pull request and tick each item. Each step links to the rule that governs it.

## Steps

- [ ] 1. Load the `orcon-ui` skill and read the reference sections that apply to this screen. Do not work from memory ([components.md#screens](components.md#screens), [foundations.md#layout](foundations.md#layout)).
- [ ] 2. Build the screen from existing `@orcon/ui` components and templates; create a new component only when none fits, following the component checklist ([component-checklist.md](component-checklist.md), [components.md#inventory](components.md#inventory)).
- [ ] 3. Declare the states of the screen (default, loading, empty, error, plus domain states such as UNCERTAIN or awaiting a person) and make sure every component pair used is declared ([components.md#states](components.md#states), [tokens.md#contrast-pairs](tokens.md#contrast-pairs)).
- [ ] 4. If a token is missing, add it in `@orcon/tokens` for both themes and run the contrast test; never put a value in the screen ([tokens.md#adding-a-token](tokens.md#adding-a-token)).
- [ ] 5. Implement with token utilities, Radix-based components, Lucide icons and `react-intl` strings; templates use sample data fixtures and make no HTTP calls ([tokens.md#usage-in-code](tokens.md#usage-in-code), [content-and-i18n.md#strings](content-and-i18n.md#strings)).
- [ ] 6. Write one story per screen state plus `Matrix`; check light and dark, comfortable and compact, phone (390×844) and desktop (1440×900), tablet (1024×768) for the editor, and the pseudo-locale `xx-LS` ([foundations.md#layout](foundations.md#layout), [content-and-i18n.md#expansion](content-and-i18n.md#expansion)).
- [ ] 7. Run `sh .agents/skills/orcon-ui/scripts/verify.sh` until it exits 0.
- [ ] 8. Review the screenshots against `brand/reference/*.png` following the review protocol and attach the findings table to the pull request ([review-protocol.md#capture](review-protocol.md#capture), [review-protocol.md#findings-table](review-protocol.md#findings-table)).
- [ ] 9. Do the manual keyboard and screen-reader pass and record it in the pull request ([accessibility.md#manual-pass](accessibility.md#manual-pass)).
- [ ] 10. For a primary screen template (S1 to S9), add or reset its row to `Pending` in "Owner review status" of `docs/design-system/README.md`, using `assets/review-record-template.md` ([review-protocol.md#owner-gate](review-protocol.md#owner-gate)).

## Screen rule checks

- [ ] App shell, landmarks, skip link and navigation follow the layout rules for each breakpoint ([foundations.md#layout](foundations.md#layout), [components.md#inventory](components.md#inventory)).
- [ ] Phone: operational surfaces are fully usable; the canvas is read-only below md ([canvas.md#mobile](canvas.md#mobile)).
- [ ] Errors use `ProblemAlert`, `FormField` and `ErrorSummary`; input is never cleared on error ([components.md#forms-and-errors](components.md#forms-and-errors)).
- [ ] Notifications and live announcements are polite and coalesced; toasts are never used for decisions ([components.md#notifications](components.md#notifications), [accessibility.md#live-regions](accessibility.md#live-regions)).
- [ ] Protected data shows `RedactedValue`; copy never copies hidden values ([components.md#redaction](components.md#redaction)).
- [ ] UNCERTAIN is never shown as failure or success and "repeat" is never the primary action ([components.md#status](components.md#status)).
- [ ] Dates, numbers and relative times use Intl ([content-and-i18n.md#formatting](content-and-i18n.md#formatting)).
- [ ] Mascot only in allowed contexts (never on error pages, security warnings, canvas or dense views) ([foundations.md#mascot](foundations.md#mascot)).
- [ ] Focus is never hidden by sticky UI; zoom to 200 % and reflow work ([accessibility.md#focus](accessibility.md#focus), [accessibility.md#manual-pass](accessibility.md#manual-pass)).
- [ ] Comments, stories, commit messages and PR text use neutral contributor wording (no coding-agent product names, no AI authorship trailers).
