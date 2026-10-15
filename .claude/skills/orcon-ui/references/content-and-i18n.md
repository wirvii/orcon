<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Content and i18n

Status: Accepted | Version: 1.3.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. The UI is translatable from day one (owner decision Q9). Messages use react-intl / FormatJS with ICU MessageFormat.

## Voice

1. Plain, specific and calm. Say what happened, why, and what the person can do next.
2. No blame ("The connection was refused", not "You entered a wrong password").
3. No jokes, puns or mascot language in errors, warnings or security messages.
4. Sentence case for every label, title, button and menu item ("Create flow", not "Create Flow").
5. Buttons start with a verb that names the result ("Delete flow", "Approve", "Continue execution"); avoid "OK" and "Yes" in confirmations.
6. Numbers in text use digits and Intl formatting.
7. Never expose internal identifiers as the only explanation; codes appear next to a human message.
8. UNCERTAIN uses its fixed wording and is never described as failed or succeeded ([components.md#status](components.md#status)).

## Strings

1. No user-visible literal in code. This includes JSX text and the visible or announced attributes `aria-label`, `aria-description`, `title`, `placeholder`, `alt` and `label`. The hard-coded strings check enforces it.
2. Every string is rendered through `FormattedMessage` or `intl.formatMessage`.
3. Message ids follow `area.component.purpose`, lowercase with dots and hyphens (for example `executions.status-badge.uncertain-description`, `settings.appearance.theme-label`). Server error codes use `problem.<code>`.
4. Every message has a `description` for translators explaining where it appears and what placeholders mean.
5. Exceptions that are not translated: product name "Orcon", connector names, user content, identifiers, code and JSON.
6. Strings in stories and fixtures that represent user content MAY be literals in fixture files; UI chrome in stories still comes from the catalogs.

## Catalogs

1. Catalogs live in `packages/js/ui/src/i18n/en.json` (source) and `es.json`. Both are complete in V1.
2. `pnpm i18n:check` fails when a key is missing in either language, when a key is unused, or when a message is not valid ICU.
3. Locale selection: the user preference if set; otherwise the first entry of `navigator.languages` that matches `en` or `es` (language subtag match); otherwise `en`.
4. The `lang` attribute of `<html>` always matches the active locale.
5. Adding a language later requires only a new catalog file and an entry in LocaleSwitcher.

## ICU

1. Plurals and variants use ICU `plural` and `select` (`{count, plural, one {# item} other {# items}}`). Spanish plural rules come from Intl, never from hand-written conditions.
2. Never concatenate translated fragments, and never build sentences from pieces in code; each sentence is one message with placeholders.
3. Rich text (links, emphasis) inside a message uses react-intl rich text tags, not string splitting.
4. Placeholders are named (`{flowName}`), never positional.

## Formatting

1. Dates, times, relative times, numbers, percentages, currency and durations are formatted only through Intl (react-intl components or `intl` methods, or the DateTime/RelativeTime/NumberFormat components).
2. ISO 8601 timestamps are shown on hover and are what copy actions copy.
3. Times use the browser time zone; the zone is shown when it is ambiguous (for example in schedules, approvals with expiry and audit views).
4. Never format dates or numbers by hand (`toFixed`, string templates, fixed separators).

## Expansion

1. Every layout tolerates text at least 30 % longer than English without overflow, overlap or clipping.
2. Storybook and the visual tests include the pseudo-locale `xx-LS`, which lengthens every message.
3. Containers wrap text by default. Truncation is allowed only when declared with `data-truncate`, and the full text is available in a tooltip and in the accessible name.
4. Fixed widths for text containers are not allowed; use min/max widths and wrapping.

## Terminology

Use these terms consistently in both languages:

| English | Spanish | Meaning |
|---|---|---|
| flow | flujo | A workflow definition |
| execution | ejecución | One run of a flow |
| node | nodo | One step of a flow |
| item | elemento | One unit of data processed by a node |
| uncertain (status label) | para revisar | The external system may or may not have applied an operation; a person must review it |
| uncertain (adjective in descriptions) | incierto | The external system may or may not have applied an operation |
| awaiting a person | esperando a una persona | Work paused until someone decides |
| needs action | requiere acción | A suspended execution that needs a decision |
| approval | aprobación | A decision to allow or deny a tool call or step |
| connector | conector | Integration with an external service |
| credential | credencial | Stored secret used by a connector |
| retry | reintentar | Run a failed attempt again |
| continue | continuar | Resume a paused execution |
| cancel | cancelar | Stop an execution |
| pinned data | datos fijados | Output kept as the node's result so later runs reuse it |
| sticky note | nota adhesiva | A free-text note placed on the canvas |
| node details | detalle del nodo | The view that shows a node's input, parameters and output |
| run step | ejecutar paso | Run one node using the data it already has |
| tidy up | ordenar | Align nodes to the grid in columns |
