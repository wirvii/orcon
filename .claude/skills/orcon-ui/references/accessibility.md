<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Accessibility

Status: Accepted | Version: 1.1.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119.

## Target

1. Conformance target: WCAG 2.2 level AA for every component, screen template and screen of `apps/web`.
2. Automated: zero axe violations (WCAG A and AA, 2.0 to 2.2 rule sets) in every story in light and dark, every screen template and the `apps/web` screens.
3. Manual: the pass of [#manual-pass](#manual-pass) for every screen template.
4. Status is never conveyed by color alone (WCAG 1.4.1): icon + text always ([components.md#status](components.md#status)).
5. Use native HTML semantics first; ARIA only where no native element exists, following the WAI-ARIA Authoring Practices patterns.

## Contrast

1. Contrast is guaranteed by tokens: only certified pairs may be used ([tokens.md#contrast-pairs](tokens.md#contrast-pairs)). Text ≥ 4.5:1 (1.4.3); non-text UI that conveys information ≥ 3:1 (1.4.11).
2. axe also checks the rendered contrast; both checks must pass.
3. Text over images or gradients is not allowed.
4. Focus indicators meet 3:1 against adjacent colors ([#focus](#focus)).

## Focus

1. Every interactive element shows a visible `:focus-visible` ring: 2 px `focus.ring`, 2 px offset. Never remove the outline without this replacement.
2. A focused element is never hidden by sticky headers, toolbars, banners or toasts (2.4.11 Focus Not Obscured); scroll padding accounts for sticky UI.
3. Focus order follows the visual and reading order.
4. Dialogs, sheets and popovers move focus inside on open and return it to the trigger on close; dialogs trap focus.
5. A skip link to the main content is the first focusable element of AppShell.
6. Focus is never moved by live updates or notifications.

## Keyboard

1. Every pointer interaction has a keyboard path (2.1.1), including the canvas ([canvas.md#keyboard](canvas.md#keyboard)).
2. No keyboard traps except intentional dialog focus traps, which close with Escape.
3. Single-character shortcuts (canvas `A`, `C`, `?`) work only when the canvas has focus and are listed in the shortcut help (2.1.4).
4. Composite widgets (menus, tabs, radio groups, listboxes, tree outline) use arrow keys inside and a single Tab stop.
5. Every row of [canvas.md#parity](canvas.md#parity) has a keyboard script that reaches the same end state as the pointer script within the same number of steps; inside a focused node, Tab reaches the toolbar buttons and then the output ports, and a focused connection offers its actions ([canvas.md#keyboard](canvas.md#keyboard)).

## Targets

1. Coarse pointers (`@media (pointer: coarse)`): every interactive control has a hit area of at least 44×44 px in both densities.
2. Fine pointers: at least 24×24 px, or enough spacing per WCAG 2.5.8.
3. Exception: links inside running prose (container `data-prose`), per the inline exception of 2.5.8.
4. The targets check verifies 44×44 px under touch emulation and the row heights of each density.
5. Dragging movements have a single-pointer alternative (2.5.7): canvas nodes move with keyboard steps; connections use the Connect dialog; sortable lists have move up/down actions.

## Live regions

1. One application-level `aria-live="polite"` region for status changes and notifications.
2. Announcements are coalesced: at most one every 5 seconds, summarizing what changed ("3 executions changed status").
3. `aria-live="assertive"` is reserved for errors that block the current task.
4. Announcements never move focus and never repeat unchanged information.

## Motion

1. Respect `prefers-reduced-motion: reduce`: all transition durations become 0 ms and the running indicator is static (2.3.3).
2. No flashing content (2.3.1), no parallax, no auto-playing animation.
3. Status remains understandable without motion.

## Forms

1. Every control has a programmatic label; descriptions and errors are linked with `aria-describedby`; invalid fields have `aria-invalid="true"`.
2. Errors are identified in text next to the field and in a focusable ErrorSummary (3.3.1, 3.3.3) ([components.md#forms-and-errors](components.md#forms-and-errors)).
3. Consistent help (3.2.6): help and support links appear in the same place on every screen.
4. Redundant entry (3.3.7): information already entered in the same process is pre-filled or selectable, never asked again.
5. Accessible authentication (3.3.8): sign-in never requires a cognitive test; password managers and paste are allowed.
6. Input purpose (1.3.5): personal data fields use the correct `autocomplete` values.
7. Time limits (approval expiry) are shown in text with relative time and announced before they run out.

## Manual pass

For every screen template (S1 to S9) and every new screen, record in the pull request:

1. Keyboard-only walkthrough: reach and operate every control, verify focus order and visibility, no traps, shortcuts listed.
2. Screen reader pass with VoiceOver (macOS, Safari) and NVDA (Windows, Firefox or Chrome): landmarks, headings, names, roles, states, status announcements, error announcements.
3. Zoom to 200 % and reflow at 320 CSS px width (1.4.10) without loss of content or function.
4. Reduced motion enabled: no motion, status still clear.
5. Result line per screen: tester, date, assistive technology and version, findings (or "none").
