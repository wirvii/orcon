<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Canvas

Status: Proposed — pending owner approval | Version: 1.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3; editor style A, Enmienda 1)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. The workflow canvas is built on `@xyflow/react` with Orcon primitives from `@orcon/ui` (`src/canvas/`, `src/editor/`). All canvas values are CSS px at 100 % zoom and density-independent (canvas geometry scales with zoom only; editor panels follow density). Token names are those of [tokens.md#canvas-tokens](tokens.md#canvas-tokens). The primitives are CanvasNode, NodeStatusBadge, NodeStatusChip, PinnedBadge, NodePort, PortAddButton, OutputStub, CanvasEdge, EdgeLabel, EdgeActions, NodeToolbar, StickyNote, CanvasControls, CanvasMinimap, AddFirstStep, AiBlock, NodePalettePanel and CanvasOutline; the editor components are NodeDetailsView, PaneSplitter, DataPane, DataTableView, JsonTreeView and SchemaView.

## Style

Style A "familiar", chosen by the owner: square icon tiles with the name below, a layout and interactions familiar to users of the n8n editor, Orcon's own palette, icons and brand. Never copy n8n code, CSS, icons, assets or branding. The decision mockup is kept at `docs/design-system/mockups/editor-style-a.html` (record only; this file governs).

## Layout

Left to right. Editor frame: top bar (flow name, last-run status chip, Save, primary "Execute workflow" with Play) + canvas + node details view. Zoom controls bottom-left; minimap bottom-right; nothing else floats over the canvas except node toolbars, edge actions, the node palette panel and toasts. Default column pitch 200 (tile 100 + gap 100); rows align to the grid.

## Background

`canvas.bg` with a dot grid: dot radius 1.4 in `canvas.grid`, spacing 20, scaled with zoom. Snap grid 20; node positions are multiples of 20.

## Node tile

Every non-trigger node is a 100 × 100 tile, radius `2xl` (18), fill `canvas.node.bg`, 1 px `canvas.node.border`, shadow sm in light, border only in dark. A centered 40 × 40 icon box: connector icons through `ConnectorIcon variant="tile"` (`<img>`, no inner frame); built-in node icons are Lucide at 36, stroke 2, `fg.default`. Height grows for more than two outputs: `max(100, 26 × outputs + 22)`; width stays 100. The tile is a button with the node's accessible name (`#screen-reader`).

## Logic nodes

If, switch, filter, merge, loop and wait use the same tile (no pill shape), icon in `fg.link`. Outputs are evenly spaced on the right edge (two outputs at y 24 and 76; more at a 26 pitch, centered). Each output label ("true", "false", case names) sits inside the tile, right-aligned 10 from the edge, mono xs 12/16 600 `fg.muted`, vertically centered on its port. Merge inputs at y 24 and 76 on the left edge.

## Trigger node

The tile with left corners `radius.full` (the left side is a half circle) and right corners 18; no input port; a 22 trigger mark centered on the left edge (overlapping by 11): `canvas.node.bg` fill, 2 px `fg.accent` border, Lucide `Zap` 12 in `fg.accent`; built-in trigger icons in `fg.accent`. Its toolbar starts with "Execute workflow".

## AI blocks

An agent or chain node uses the wide tile 200 × 100. Its configuration blocks (`ai.model`, `ai.memory`, `ai.tools[]`, `ai.outputParser`, retrieval) are 48 circles below the label block, joined to the tile's bottom edge by 2 px dashed `canvas.edge` lines, each with a mono xs label ("Model", "Memory", "Tools", "Parser"); an unset block is a dashed circle with `Plus`. Blocks are not nodes: not separately selectable, not flow ports, not executable; activating one opens the Agent tab of the node details view.

## Labels

Centered below the tile with a 6 gap, max width 150: name (sm 13/18 600 `fg.default`, up to two lines then ellipsis; full name in tooltip and accessible name), operation subtitle (xs 12/16 `fg.muted`, one line, ellipsis), then up to two chips 2 below (status chip first, then "Pinned"). Labels never go inside the tile. Labels pass pointer events to the canvas except double-click on the name, which starts inline rename (same as F2).

## Ports

Inputs centered on the left edge (evenly spaced when several), outputs on the right edge. A port is a 10 circle, fill `canvas.node.bg`, 2 px `canvas.port` border, hit area extended by 9 on each side (≥ 28; ≥ 44 on coarse pointers). An output port is a button: on hover or focus it grows to 24 in `duration.fast`, fills `action.primary.bg` (border too) and shows `Plus` in `action.primary.fg`; activating it opens the node palette to insert a node connected to that output; dragging from it starts a connection. While connecting, valid input ports grow to 14 with an `action.primary.bg` border; an invalid target shows `not-allowed` and a tooltip with the reason (rules in SPEC-014 R-ED-CAN-01). On a selected node the port borders are `canvas.selection`.

## Add node

(1) output-port plus (`#ports`); (2) dangling output stub: an output with no edge shows a 30 long, 2 px dashed `canvas.edge` line ending in a 22 dashed square (radius 6, `canvas.node.bg`) with `Plus` in `fg.muted`; hover or focus turns border and icon `action.primary.bg`; (3) edge insert (`#edges`); (4) empty canvas: one centered 100 dashed tile "Add first step" (`Plus` 36, label below) that opens the palette filtered to triggers; (5) key `A`. The node palette is a right-side panel 360 wide, `surface.raised`, shadow lg, search field focused on open, grouped list, arrow keys move, Enter inserts, Esc closes (index, search and grouping: SPEC-014 R-ED-PAL). A newly inserted node lands one column pitch to the right of its source, same row, and is selected.

## Edges

Cubic bezier with horizontal tangents (control offset `max(30, Δx / 2)`), 2 px, round caps, a 7 filled arrowhead at the target. States: design time (no run data) solid `canvas.edge`; succeeded solid `canvas.edge-active`; running dashed 7/6 in `status.active.fg` with a dash-offset flow (`motion.loop.edge`, static with reduced motion); failed dotted 2/5 in `status.danger.fg`; uncertain dashed 8/5 in `status.attention.fg`; not run in this execution dashed 3/5 in `canvas.edge`. Loop-back (`BACK`) edges route below the nodes and carry a `Repeat` chip. After any run, a label chip sits at the midpoint: pill, min height 20, padding 0 8, `canvas.node.bg`, 1 px border and text in the state color (`status.success.fg`, `status.active.fg`, `status.danger.fg`, `status.attention.fg`), xs 12/16 500, shadow sm, text = ICU plural item count ("1 item", "3 items", "0 items"), "unknown" when uncertain, " · pinned" appended when the source is pinned. Hover or focus on an edge: stroke 3 and an action bar at the midpoint (the label moves 16 up) with "Add node here" (`Plus`) and "Delete connection" (`Trash2`), 24 buttons (44 on coarse pointers) on `surface.raised`. Color is never the only signal: dash pattern and label text differ per state.

## Execution states

Badge = 22 circle at the tile's top-right (offset −8, −8), fill = family fg, glyph = the §6.7 icon at 14 stroke 3 in family bg, 2 px ring in `canvas.bg`. Chip = small `NodeStatusChip` (pill, xs 12/16 600, family bg/fg/border, icon + text) under the name.

| State | Tile | Badge | Chip | Incoming edge |
|---|---|---|---|---|
| Not executed (design time) | default | none | none | design time |
| Running | 1 px `status.active.fg` border + run ring 6 outside the tile (conic sweep, `motion.loop.ring`; reduced motion: static segmented ring) | LoaderCircle (`motion.loop.spin`; static with reduced motion) | "Running" | running |
| Succeeded | default | CircleCheck glyph | none: the quiet state (text in tooltip, outline and accessible name) | succeeded |
| Succeeded with errors | 2 px `status.warning.fg` | CircleAlert | "Succeeded with errors" | succeeded |
| Failed | 2 px `status.danger.fg`, fill `status.danger.bg` | CircleX glyph | "Failed", or "Failed · <short code>" when the error has one | failed |
| Uncertain | 2 px `status.attention.fg` | ShieldQuestion | "Uncertain", or "<n> uncertain" for items | uncertain |
| Waiting / Needs action / Awaiting a person | 2 px family border | §6.7 icon | §6.7 label | running pattern without flow animation |
| Canceled | default | Ban | "Canceled" | not run |
| Not run (execution ended without it) | default; icon in `fg.subtle` | none | "Not run" (debugger and execution detail only) | not run |
| Disabled | 1 px dashed `canvas.edge`, fill `surface.sunken`, icon `fg.subtle` | none | "Disabled" (Ban, dashed chip border) | not run |

Succeeded is the only state without visible text on the canvas; the check glyph is a shape signal (WCAG 1.4.1 met by shape) and the text is in the tooltip, the outline and the accessible name. Every other non-default state shows icon + text. Only the run ring, the running spinner and the running edge animate.

## Pinned

A 22 badge at the tile's top-left (offset −8, −8), `surface.raised` fill, 2 px `canvas.bg` ring, Lucide `Pin` 12 in `fg.link`, plus a neutral chip "Pinned" (Pin icon). In a test run a pinned node shows Succeeded and its outgoing label adds " · pinned".

## Placeholder node

`orcon.unsupported@1` renders as a tile with a 1 px dashed `status.warning.fg` border, `surface.sunken` fill, `CircleHelp` in `fg.muted`, chip "Not supported" (warning family); always disabled; no run button in its toolbar.

## Diff overlay

(compare mode) added = 2 px solid `status.success.fg` + chip "Added" (Plus); changed = 2 px dashed `status.warning.fg` + chip "Changed" (PenLine); removed = 2 px dotted `status.danger.fg`, icon in `fg.subtle` + chip "Removed" (Minus). Never color alone.

## Selection and hover

Hover: border `canvas.edge`, grab cursor, toolbar shown. Selection: 2 px `canvas.selection` outline 4 outside, following the tile shape; port borders `canvas.selection`. Keyboard focus: 2 px `focus.ring`, offset 2 (distinct from selection; both can show). Multi-select with Shift+click and marquee (1 px `canvas.selection` border, `canvas.marquee.bg` fill). Stacking: hovered or focused above selected, selected above others.

## Node toolbar

Floats 40 above the tile, centered: `surface.raised`, 1 px `canvas.node.border`, radius `md` (8), shadow md, padding 3, gap 2. Buttons 28 (44 on coarse pointers), radius 6, icon 16 `fg.default`, hover `surface.sunken`. Order: [Execute workflow — trigger only], Run step (`Play`), Disable/Enable (`Power`), Delete (`Trash2`; hover `status.danger.bg` / `status.danger.fg`), More (`Ellipsis` → Rename, Duplicate, Pin/Unpin, Copy, Open details). Shown on hover, focus-within and selection, opacity transition `duration.fast`; a 10 invisible bridge keeps it reachable from the tile. Each button has an accessible name and a tooltip with its shortcut.

## Sticky notes

Behind nodes; radius 8; padding 10 12; 1 px border `canvas.sticky.N.border`; fill `canvas.sticky.N.bg` (default N = 1); header line with `StickyNote` 14 and optional title (xs 12/16 600 `canvas.sticky.N.accent`); body xs 12/16 `fg.default`, Markdown subset without raw HTML; resize grip bottom-right (10, 2 px accent corner); minimum 160 × 80; six decorative colors only, never status colors.

## Zoom and minimap

Zoom between `canvas.zoom.min` (20 %) and `canvas.zoom.max` (400 %); Ctrl or Cmd + wheel and pinch zoom around the pointer; wheel and two-finger scroll pan; Space + drag and middle-button drag pan. Controls bottom-left, 12 from the edges: Fit view (`Maximize`), Zoom in (`ZoomIn`), Zoom out (`ZoomOut`), Reset to 100 % (shown only when zoom ≠ 100 %), Tidy up (`Wand`, behavior in SPEC-014); IconButtons 32 (44 on coarse pointers) on `surface.raised`, `border.default`, radius 8, tooltip with shortcut. Minimap bottom-right 200 × 120, `surface.raised`, radius 8, shadow md: nodes as rectangles in `canvas.node.border`, selected in `canvas.selection`, viewport rectangle with a 2 px `canvas.selection` border; shown while panning or zooming and for 1000 ms after, and while hovered; click or drag on it moves the viewport; hidden below `md`.

## Level of detail

Below `canvas.lod.detail` (60 %): subtitle, edge labels, output labels, toolbars and edge actions hidden; name and chips counter-scaled so their on-screen size is ≥ 10; badges scaled 1.4. Below `canvas.lod.minimal` (35 %): labels and chips hidden; tiles, badges and edges only; names stay in tooltips and the outline.

## Node details view

Opens on double-click, Enter, or "Open details"; ≥ `lg`: a dialog over the canvas (`role="dialog"`, `aria-modal="true"`, scrim `overlay.scrim`), inset 24, `surface.default`, radius 12, shadow lg. Header 56 high: "Back to canvas" (`ArrowLeft`; Esc), node icon in a 36 `surface.sunken` square radius 10, name (md 16/24 600, renameable) and operation subtitle (xs `fg.muted`), status chip, primary "Execute step" (`Play`) at the right end. Body: three panes Input | Parameters | Output; Parameters width `clamp(size.editor.params-min, 34 %, size.editor.params-max)`; Input and Output share the rest equally, each ≥ `size.editor.pane-min`; splitters are `role="separator"` with `aria-valuenow`, draggable and resizable with arrow keys (20 per press). Trigger nodes have no Input pane. Parameters pane: tabs Parameters / Settings / Docs (+ Agent for AI nodes) with a 2 px `canvas.selection` underline on the current tab; fields are `FormField` (label sm 13/18 500, control height `h-control`, hint xs `fg.muted`); expression fields mono 13/20 with the Fixed / Expression switch at the right of the label; when the last run left the node failed or uncertain, a status alert heads the form (family bg/fg/border, icon + title + text + actions; uncertain never offers "run again" as the primary action). `md`–`lg`: no dialog; the editor frame docks the Parameters pane on the right (`size.editor.docked-panel`) and the data panel at the bottom across the full width, with an Input / Output switch (the mockup arrangement). Closing returns focus to the node tile.

## Data panel

Every data pane (Input, Output, and the docked bottom panel) has a header — title; source and run selector ("from HTTP Request", "Run 2 of 5"); output-port tabs with counts for multi-output nodes; a segmented control Table | JSON | Schema (pressed segment: `fg.default` fill, `bg.app` text, 600); ICU item count ("3 items"); in Output also Pin/Unpin and Edit — and a body. Table: sticky header in `surface.sunken`, xs 12/16 600 `fg.muted`; "#" column 40 wide in `fg.subtle`; cells mono 13/20; `null` in `fg.subtle`; nested objects collapsed as "{…} 3 keys" and expandable; row height `h-row`; virtualized; horizontally scrollable region with an accessible name. JSON: collapsible tree, mono 13/20; keys `fg.default`, strings `fg.accent`, numbers `fg.link`, booleans `fg.default` 600, null `fg.subtle`; no other hues. Schema: field tree with a neutral type `Badge` and an example value (mono, ≤ 60 characters, masked when sensitive). Empty states: Input "No input data yet" + "Execute previous nodes"; Output "Execute this step to see output" + "Execute step". Pinned output: neutral `Banner` with `Pin` icon "This data is pinned" + Unpin. Protected values render `RedactedValue`; values masked in history render "Hidden in history"; copy never copies hidden values. Binary items: name, media type, size and download.

## Keyboard

Tab enters the canvas; arrows move focus spatially between nodes (with no node focused, they pan); inside a focused node, Tab reaches the toolbar buttons and then the output ports; Enter opens the node details view; Esc closes it and returns focus; Shift + arrows move the selection one grid step (20), Alt + Shift + arrows four steps; `A` node palette; `C` Connect dialog; Delete with confirmation for multi-select and an undo toast; `+` / `-` / `0` / `1` zoom in / out / 100 % / fit; Ctrl or Cmd + Z / Y undo / redo; `?` lists shortcuts (translatable). The full map is SPEC-014 Annex B, which extends this list without changing it. With two nodes selected, `C` connects the one on the left to the other through the first free output and input, and opens the Connect dialog when that is not possible; Ctrl or Cmd + `A` selects every node; with focus on the canvas itself the arrow keys pan; Ctrl or Cmd + Shift + arrows resize a focused sticky note by one grid step; `R` runs the focused step, `D` disables or enables it, `P` pins or unpins its output, F2 renames it and Shift + `S` adds a sticky note. On a focused connection Enter opens the node palette to insert a node on it and Delete removes it.

## Screen reader

`CanvasOutline` lists nodes, connections (with item counts and state), sticky notes and statuses. Node accessible name: "<name>, <operation>. Status: <status>[, pinned][, disabled]. <n> inputs, <m> outputs." Edge accessible name: "<source> (<output>) to <target>: <count>, <state>". Status changes go through the polite, coalesced live region. Badges are `aria-hidden`; their meaning is in the name.

## Mobile

Below `md` the canvas is read-only: tiles, labels, badges, chips and edges as above; no toolbars, port plus buttons, stubs, edge actions, minimap, palette or node details editing; tapping a node opens a bottom sheet with status, counts and a link to execution detail.

## Performance

60 fps target for pan, zoom and drag with 500 nodes on a mid-range laptop (SPEC-010 PF-4; measured by SPEC-014 R-ED-PERF-02); no per-frame React state for edges; animations only on running elements; level of detail reduces the rendered DOM at low zoom.

## Parity

The V1 bar is that every row below is equal to or better than the n8n editor experience, observed as a user (nothing copied). For each row owned by SPEC-007 there is a Storybook story `Editor/Parity/<ID>` with a pointer script and a keyboard script run by `test:parity` (end state asserted, steps ≤ "Max steps"), and visual baselines of the end state in light and dark. Steps count the commit actions needed once the target is reached (a click, a drag, a key chord); moving the pointer or the focus onto the target, and typing the content of a field, are not steps. Rows owned by SPEC-014 get the same in SPEC-014. The owner records Equal, Better or "Worse — changes requested" per row in `docs/design-system/README.md` ("Editor parity review").

| ID | Interaction | Orcon style A | Max steps | Owner |
|---|---|---|---|---|
| P-01 | First step on an empty canvas | "Add first step" tile → palette filtered to triggers → pick | 2 | SPEC-007 |
| P-02 | Add a node after a node | output-port plus → pick; placed one column right, connected, selected | 2 | SPEC-007 |
| P-03 | Add after a dangling output | stub plus → pick | 2 | SPEC-007 |
| P-04 | Insert a node on a connection | edge hover → "Add node here" → pick; edge split and reconnected | 2 | SPEC-007 |
| P-05 | Delete a connection | edge hover → "Delete connection" | 1 | SPEC-007 |
| P-06 | Connect two nodes | drag output port to input port or node body; valid/invalid feedback | 1 | SPEC-007 (rules SPEC-014) |
| P-07 | Move and multi-select | drag with 20 snap; Shift+click; marquee; drag group | 1 per move | SPEC-007 |
| P-08 | Node quick actions | hover toolbar: run step, disable, delete, more | 1 | SPEC-007 |
| P-09 | Select and open a node | click selects; double-click or Enter opens the node details view | 1 | SPEC-007 |
| P-10 | Node details view | Input / Parameters / Output panes; Back or Esc returns focus to the node | 1 | SPEC-007 |
| P-11 | Switch data view | Table / JSON / Schema per pane | 1 | SPEC-007 |
| P-12 | Map an input field into a parameter | drag a field (or Enter on it) into an expression | 1 | SPEC-014 |
| P-13 | Execute one step | Run step (toolbar or details view) → running → result; output fills | 1 | SPEC-007 visuals; SPEC-014 behavior |
| P-14 | Execute the workflow | "Execute workflow" → states propagate; edge item counts | 1 | SPEC-007 visuals; SPEC-014 behavior |
| P-15 | Pin output data | Pin in Output → pinned badge, chip and banner | 1 | SPEC-007 visuals; SPEC-014 storage |
| P-16 | Disable a node | toolbar or `D` → dashed tile, "Disabled" chip | 1 | SPEC-007 |
| P-17 | Read a failed node | red border and fill, badge, "Failed · code" chip; error in Output | 0 (visible) | SPEC-007 |
| P-18 | Read an uncertain node (Orcon improvement) | violet border, badge, "Uncertain" chip; alert with actions in Parameters | 0 (visible) | SPEC-007 |
| P-19 | Sticky notes | add (Shift+S), edit, resize, recolor | 1 per action | SPEC-007 visuals; SPEC-014 store |
| P-20 | Zoom | controls, `+`/`-`/`0`/`1`, Ctrl or Cmd + wheel, pinch; level of detail | 1 | SPEC-007 |
| P-21 | Minimap | appears while panning; click or drag to move the view | 1 | SPEC-007 |
| P-22 | Pan | wheel or trackpad, Space + drag, middle-button drag | 1 | SPEC-007 |
| P-23 | Copy, paste, duplicate | Ctrl or Cmd + C / V / D, Orcon and n8n JSON | 1 | SPEC-014 |
| P-24 | Undo and redo | Ctrl or Cmd + Z / Shift+Z / Y | 1 | SPEC-014 |
| P-25 | Rename a node | double-click the name or F2, inline under the tile | 2 | SPEC-007 visuals; SPEC-014 reference rewrite |
| P-26 | Tidy up | control or Shift+Alt+T | 1 | SPEC-014 |
| P-27 | Find a node type | palette search, categories, keyboard navigation | 2 | SPEC-007 panel; SPEC-014 index |
| P-28 | Multi-output node | labeled outputs inside the tile; tile grows beyond two outputs | 0 (visible) | SPEC-007 |
| P-29 | Trigger node | half-round left side, lightning mark; "Execute workflow" in its toolbar | 1 | SPEC-007 |
| P-30 | Same experience in dark theme and by keyboard | every SPEC-007 row above also in dark and with its keyboard script | as each row | SPEC-007 |
