<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# State

Status: Proposed — pending owner approval

This document defines where each kind of state lives and how the flow editor store works: one store per open editor, changed only by named actions, with undo and redo built from Immer patches (ADR 0078). Rule IDs (`ST-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Where state lives

- **ST-1** Where state lives:

  | State | Lives in |
  |---|---|
  | Server data | TanStack Query (`data-and-live-updates.md`) |
  | Shareable view state (filters, sort, page token, tab, selection) | URL search (`routing.md` RT-3) |
  | Ephemeral UI state | `useState` / `useReducer` in the component |
  | The open flow being edited and its history | the editor store |
  | User preferences (theme, density, locale) | the `orcon-ui` providers, persisted through `app/boundary/storage` |

- **ST-2** Zustand and Immer only in `features/editor/store/**`.
- **ST-11** React context carries only stable references (store, query client), never frequently changing values.

## The editor store

- **ST-3** One store per open editor: `createEditorStore(initial)` with `createStore` from `zustand/vanilla`, provided through React context; no module-level singleton.
- **ST-4** Shape: `document` (working copy of the draft, `orcon/flow@1`), `baseRevision` (the server `draftRevision`), `selection`, `history { past, future }` of Immer patch pairs, `clipboardFallback`.
- **ST-5** Named actions are the only writers: `addNode`, `removeNodes`, `connect`, `disconnect`, `moveNodes`, `updateNodeConfig`, `renameNode`, `paste`, `undo`, `redo`, `select`. Each user action is one history entry (a drag records one entry at the end; intermediate positions are transient). History keeps the last 100 entries; undo restores the selection.
- **ST-6** Components subscribe with narrow selectors (`useShallow` or per-id selectors) and call actions; never `setState` from components.
- **ST-7** The store is tested without React: every action, and for every action `apply → undo` returns a deep-equal document and `undo → redo` returns the post-action document.

## Clipboard and saving

- **ST-8** Copy writes an `orcon/flow-fragment@1` document through `app/boundary/clipboard`; paste parses it with Zod, rejects invalid content with a message and assigns new ids.
- **ST-9** Saving sends the working document with `If-Match: baseRevision`; success updates `baseRevision` and keeps history; `412` opens the conflict flow; server data never overwrites the working copy silently.

## Canvas

- **ST-10** React Flow is controlled by the store: nodes and edges are memoized selectors; `onNodesChange`/`onEdgesChange`/`onConnect` translate into store actions. The look of the canvas follows [canvas](../../orcon-ui/references/canvas.md#style).
