<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Components

Status: Accepted | Version: 1.2.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. Components live in `@orcon/ui` (`packages/js/ui`). Values are never written here; they come from tokens ([tokens.md](tokens.md)).

## Contract

Every component in `@orcon/ui` MUST satisfy all of the following:

1. **Foundation**: built on a Radix Primitive wherever one exists for the pattern (dialog, popover, menu, tabs, tooltip, select, checkbox, radio group, switch, scroll area, toast). Source derived from shadcn/ui is copied into `src/components/<name>/` and fully re-themed: no stock shadcn radius, shadow, color or focus style remains.
2. **Files**: `src/components/<name>/<Name>.tsx`, `<Name>.meta.ts` and `<Name>.stories.tsx`.
3. **Metadata**: `<Name>.meta.ts` exports `states` (the visual and domain states the component supports, see [#states](#states)) and `pairs` (every foreground/background token pair the component renders). Every pair MUST exist in `@orcon/tokens` `src/pairs.json` ([tokens.md#contrast-pairs](tokens.md#contrast-pairs)).
4. **Stories**: one story per declared state plus a `Matrix` story that renders all states together. Stories render correctly under every Storybook global: theme (light, dark), density (comfortable, compact), locale (`en`, `es`, `xx-LS`) and viewport (mobile, tablet, desktop).
5. **Keyboard**: fully operable with the keyboard, following the WAI-ARIA Authoring Practices pattern of its role.
6. **Accessible name**: every interactive element has an accessible name from the message catalog. Icon-only controls require an explicit label prop.
7. **Focus**: a visible `:focus-visible` ring of 2 px in `focus.ring` with a 2 px offset. Focus styles MUST NOT be removed or replaced by color change alone.
8. **States**: loading, empty and error states are implemented wherever they are meaningful for the component.
9. **Themes and densities**: correct in light and dark, comfortable and compact; heights and paddings come from density tokens (`h-control`, `h-row`, `px-control-x`, `gap-density`).
10. **Touch targets**: hit area ≥ 44×44 px on coarse pointers, ≥ 24×24 px otherwise ([accessibility.md#targets](accessibility.md#targets)).
11. **Strings**: every user-visible string, including `aria-label`, `title`, `placeholder` and `alt`, comes from the catalogs ([content-and-i18n.md#strings](content-and-i18n.md#strings)).
12. **Direction**: right-to-left layout is not required in V1; use logical CSS properties (`ps`, `pe`, `ms`, `me`) where Tailwind offers them so it can be added later.
13. **Wording**: code comments and stories use neutral contributor wording. They never name coding-agent products or vendors and never carry AI authorship notes.
14. **No n8n material**: no code, icons, artwork or naming copied from n8n.

## Inventory

Base components under owner review. Each one has a row in "Owner review status" in `docs/design-system/README.md`.

| Component | Requirements beyond the contract |
|---|---|
| Button | Variants primary, secondary, ghost, danger. Height from `size.control` per density. States: default, hover, focus-visible, active, disabled, loading (spinner plus label kept for width; `aria-busy`). Primary uses `action.primary.*`; danger uses `action.danger.*`. At most one primary button per view region. |
| IconButton | Requires an accessible label prop (from the catalog); shows a Tooltip with the same label on hover and focus. Square, `size.control`. |
| Link | `fg.link`, underlined in prose; external links show an external icon and a visually hidden "opens in a new tab" text when they open a new tab. |
| Input | `border.control`, height `size.control`; states default, focus-visible, disabled, read-only, invalid (`aria-invalid`). Never clears user input on error. |
| Textarea | As Input; resizable vertically; optional character counter announced politely. |
| Select | Radix Select; trigger as Input; keyboard type-ahead. |
| Combobox | Searchable list (cmdk or Radix Popover + listbox); `aria-expanded`, `aria-activedescendant`; empty result message from the catalog. |
| Checkbox | Radix Checkbox; checked, unchecked, indeterminate; label clickable. |
| RadioGroup | Radix RadioGroup; arrow keys move selection; group label required. |
| Switch | Radix Switch; immediate effect only; label states what "on" means. |
| FormField | Label, optional description, error message, required marker (visual and in the accessible name); wires `aria-describedby` to description and error and `aria-invalid` on the control. Library-agnostic (works with any form library). |
| ErrorSummary | Focusable summary at the top of a form listing every field error as a link that moves focus to the field. Receives focus after a failed submit. |
| Tooltip | Radix Tooltip; supplementary only (never the sole place of essential information); opens on hover and focus; dismissible with Escape (WCAG 1.4.13). |
| Popover | Radix Popover; focus moves inside and returns to the trigger on close. |
| DropdownMenu | Radix DropdownMenu; destructive items use the danger text token and confirm through AlertDialog. |
| Dialog | Radix Dialog; focus trap; Escape closes; title required; radius `xl`. |
| AlertDialog | Destructive confirmation. States the explicit impact in text ("Deletes 3 flows and their execution history"); the confirm button repeats the action verb; cancel receives initial focus. |
| Sheet/Drawer | Side sheet on md and wider; bottom sheet below md. Same focus rules as Dialog. |
| Tabs | Radix Tabs; arrow-key navigation; tab panel labelled by its tab. |
| Breadcrumb | `nav` landmark with label; current page `aria-current="page"`; collapses middle items below md. |
| Table + DataTable | Row height `size.row` per density; sticky header; sortable headers announce sort state (`aria-sort`); empty, loading (skeleton rows) and error rows; long text truncated only with `data-truncate` plus a tooltip; horizontal scroll inside the table, never the page. |
| Pagination | Previous/next and page numbers; current page `aria-current`; total from Intl number formatting. |
| Badge | Neutral label for counts and tags; never used for status (use StatusBadge). |
| StatusBadge | Icon + text always, color as reinforcement, per [#status](#status). Reads the normative map in `src/components/status-badge/status-map.ts`. |
| Toast | `sonner`; for confirmations and undo only; MUST NOT be used for anything that requires a decision; polite announcements; persists while hovered or focused. |
| Banner | Page-level message with status family icon + text; dismissible only when not critical. |
| ProblemAlert | Renders `application/problem+json` errors per [#forms-and-errors](#forms-and-errors). |
| Skeleton | Loading placeholder; `aria-hidden`; the region is `aria-busy` while loading. |
| Spinner | Indeterminate indicator with a visually hidden label; static under reduced motion. |
| EmptyState | Title, description, optional primary action, optional mascot slot (allowed contexts only, [foundations.md#mascot](foundations.md#mascot)). |
| ErrorBoundary fallback | Error page content: what happened, what to do, retry and copyable request id; the full-page variant shows the mascot "thinking", inline variants never do. |
| Avatar | Image with initials fallback; `alt` is the person name; decorative when the name is adjacent. |
| Kbd | Keyboard key; uses the mono font; platform-aware labels (Ctrl or Cmd) from the catalog. |
| CodeBlock | Mono font, `surface.sunken`; copy button (copies only visible, non-redacted content); horizontal scroll inside the block. |
| DateTime/RelativeTime/NumberFormat | Intl-based formatting only ([content-and-i18n.md#formatting](content-and-i18n.md#formatting)); ISO value on hover and copy. |
| RedactedValue | Per [#redaction](#redaction). |
| CommandPalette | `cmdk`; opens with Ctrl/Cmd+K; searchable; results grouped; full keyboard support. |
| Separator | Decorative divider using `border.default`; `role="separator"` only when it separates interactive groups. |
| ScrollArea | Radix ScrollArea; keyboard scrollable; focusable when it has no focusable children. |
| ConnectorIcon | Orcon frame + `<img>` connector logo per [foundations.md#iconography](foundations.md#iconography). The only way to render connector SVGs. Variant `tile` renders the bare `<img>` without the inner frame, for the icon box of a canvas node. |
| BrandMark | Approved production lockup, light and dark ([foundations.md#brand-mark](foundations.md#brand-mark)). |
| BrandHero | Sign-in brand block: descriptor lockup and the tagline as text ([foundations.md#brand-mark](foundations.md#brand-mark)). |
| Mascot | Approved board cut-outs (welcome, happy, thinking); allowed contexts only; lazy-loaded; decorative ([foundations.md#mascot](foundations.md#mascot)). |
| AppShell | Layout of [foundations.md#layout](foundations.md#layout): sidebar, rail, bottom navigation + drawer; landmarks `header`, `nav`, `main`; skip link to main content. |
| TopBar | Product mark, search or command palette trigger, NotificationBell, user menu. |
| NavItem | Icon + text; current item `aria-current="page"`; in the rail the text becomes the tooltip and accessible name. |
| NotificationBell | Count badge; accessible text "N items need your attention" (ICU plural); opens the notifications panel. |
| AttentionInboxItem | One request awaiting a person: age, cause, escalation level, who acknowledged, overdue marker (icon + text), actions per [#notifications](#notifications). |
| ThemeSwitcher | System, Light, Dark ([foundations.md#themes](foundations.md#themes)). |
| DensitySwitcher | Comfortable, Compact ([foundations.md#density](foundations.md#density)). |
| LocaleSwitcher | English, Español; each language name written in its own language. |

Canvas primitives (CanvasNode, NodeStatusBadge, NodeStatusChip, PinnedBadge, NodePort, PortAddButton, OutputStub, CanvasEdge, EdgeLabel, EdgeActions, NodeToolbar, StickyNote, CanvasControls, CanvasMinimap, AddFirstStep, AiBlock, NodePalettePanel, CanvasOutline) and editor components (NodeDetailsView, PaneSplitter, DataPane, DataTableView, JsonTreeView, SchemaView) follow [canvas.md](canvas.md) and the same contract. Each has a row in "Owner review status".

## States

1. Interaction states declared in `meta.ts` where they apply: `default`, `hover`, `focus-visible`, `active`, `disabled`, `loading`, `error`, `empty`.
2. Domain states are added as needed (for example the execution states of [#status](#status) for StatusBadge and NodeStatus).
3. Each declared state has its own story; the `Matrix` story shows them all. The story coverage check fails when a declared state has no story.
4. Disabled controls use `fg.disabled` and remain discoverable; when the reason is not obvious, a tooltip or adjacent text explains it.
5. Loading never removes content that is still valid; it marks the region `aria-busy` and shows a skeleton or spinner.

## Status

Normative map (`src/components/status-badge/status-map.ts`). State names come from the execution model; if they change when the corresponding decisions are accepted, this table changes in a minor version of the skill. Icons are Lucide names, verified against the pinned version.

| Domain state | Label (en) | Icon | Family |
|---|---|---|---|
| Execution QUEUED | Queued | Clock | neutral |
| RUNNING | Running | LoaderCircle (static with reduced motion) | active |
| WAITING | Waiting | Hourglass | active |
| SUSPENDED | Needs action | CirclePause | attention |
| CANCELING / CANCELED | Canceling / Canceled | Ban | neutral |
| COMPENSATING | Undoing | Undo2 | active |
| SUCCEEDED | Succeeded | CircleCheck | success |
| Node SUCCEEDED_WITH_ERRORS | Succeeded with errors | CircleAlert | warning |
| FAILED / item ERROR_FINAL | Failed | CircleX | danger |
| Item UNCERTAIN | Uncertain | ShieldQuestion | attention |
| Item RETRY_SCHEDULED | Retry scheduled | RotateCw | neutral |
| Item NOT_DISPATCHED | Not sent | CircleOff | neutral |
| Node not run (closed path) | Not run | CircleDashed | neutral |
| Open attention request | Awaiting a person | BellRing | attention |

Rules:

1. Status is always icon + text. Color (the family's `status.<family>.fg` and `.bg` tokens) only reinforces it.
2. The attention family (violet) is reserved for UNCERTAIN and for states that need a person (Needs action, Awaiting a person).
3. UNCERTAIN MUST NEVER be painted or named as failure or success.
4. UNCERTAIN has the fixed accessible description: "The external system may or may not have applied this operation. Review it before continuing."
5. The UNCERTAIN resolution panel shows the supervised resolutions received as data (the `actions[]` of the attention request) plus Cancel. It does not invent actions.
6. No "repeat" or "retry" action is ever the primary action for an UNCERTAIN item.
7. Actions marked `requiresRiskAck` stay disabled until the person checks an explicit acknowledgment checkbox whose text states the risk.
8. The connector's `manualCheck` text is shown verbatim (as plain text, never as HTML).
9. Only the RUNNING indicator may animate; under reduced motion it is static.

## Forms and errors

1. Server errors arrive as `application/problem+json`. `ProblemAlert` renders them:
   - Message translated by `code` with the catalog key `problem.<code>` (en and es); if no translation exists, the response `title`.
   - `detail` inside a collapsible "Technical details" section.
   - `requestId` shown with a copy button.
2. `errors[].pointer` (JSON Pointer) is mapped to the `FormField` whose `name` matches the pointer; the message appears next to the field, and the focusable `ErrorSummary` at the top of the form lists all field errors.
3. After a failed submit, focus moves to the ErrorSummary. User input is never cleared on error.
4. A `409` with type `attention-already-resolved` shows the current outcome (who resolved it, how, when) instead of a generic error.
5. Validation messages say what is wrong and how to fix it ([content-and-i18n.md#voice](content-and-i18n.md#voice)); they are not shown only by color.
6. Required fields are marked visually and in the accessible name; the meaning of the marker is stated once per form.

## Notifications

1. NotificationBell shows a count with accessible text "N items need your attention".
2. The attention inbox offers filters: mine, my groups, all. Each item shows age, cause, escalation level, who acknowledged it, and an overdue marker (icon + text).
3. Bulk actions are allowed only for items with the same cause and the same flow, and always ask for confirmation stating how many items are affected.
4. New items and status changes are announced through one `aria-live="polite"` region, coalesced to at most one announcement every 5 seconds; announcements never move focus.
5. Toasts are never used for anything that needs a decision; decisions live in the inbox, the execution detail or a dialog.

## Data display

1. Tables use `size.row` per density, a sticky header, and explicit empty, loading and error rows.
2. Numbers, dates, times, durations and relative times use the Intl-based components; ISO timestamps are available on hover and copy.
3. Identifiers, JSON, expressions and timestamps use the mono font.
4. Truncation is allowed only when declared (`data-truncate`) and the full value is reachable through a tooltip and the accessible name.
5. Large lists and catalogs are virtualized.
6. JSON previews are rendered as text (never as HTML) and respect [#redaction](#redaction).
7. Wherever execution data is shown (node input and output, execution detail), tables, JSON trees and schema views follow [canvas.md#data-panel](canvas.md#data-panel): `DataPane` with `DataTableView`, `JsonTreeView` and `SchemaView`.

## Redaction

1. Without the `execution:read-data` permission, protected values are replaced by `RedactedValue`, which shows "Hidden — you need permission to view execution data". A protected value is never shown as empty.
2. Secrets (credentials, tokens, keys) are never rendered, not even to people with data permission; they show a fixed masked placeholder.
3. Copy buttons never copy hidden or masked values.
4. Redacted values are excluded from search highlighting, tooltips and accessible names.

## Screens

Screen templates live in `src/screens/`, use sample data fixtures and make no HTTP calls. Product screens are built on them by the specification of each area.

| Template | Content | Required viewports |
|---|---|---|
| S1 | Shell + welcome/onboarding (mascot "welcome") | Phone, desktop |
| S2 | Flow list (enable/disable a flow) | Phone, desktop |
| S3 | Flow editor in style A (desktop/tablet): top bar, canvas, controls, minimap, node palette, node details view with three panes and the data panel; read-only canvas (phone); hosts the `Editor/Parity/*` stories | Desktop, tablet; phone read-only |
| S4 | Execution list | Phone, desktop |
| S5 | Execution detail: style A canvas with execution states, timeline, attempts per node, UNCERTAIN resolution panel, continue/retry/cancel | Phone, desktop |
| S6 | Human attention inbox + notifications panel | Phone, desktop |
| S7 | Approval page: tool, arguments or RedactedValue, approve / deny with reason / edit arguments, relative expiry with Intl, 409 with current outcome | Phone, desktop |
| S8 | Empty states (mascot "happy" on the first-run state only), 404 (mascot "thinking"), error page (mascot "thinking") | Phone, desktop |
| S9 | Settings › Appearance and language (theme system/light/dark, density, language) | Phone, desktop |

Every template is checked in light and dark, comfortable and compact, and the pseudo-locale, and passes the manual pass of [accessibility.md#manual-pass](accessibility.md#manual-pass).
