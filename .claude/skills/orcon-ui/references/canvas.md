# Canvas

Status: Proposed — pending owner approval | Version: 1.0 | Source: SPEC-007 (owner decisions BL-007 Q1–Q13, P1–P3)

This document is normative. The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as described in RFC 2119. The workflow canvas is built on `@xyflow/react` with Orcon primitives from `@orcon/ui` (`src/canvas/`): NodeCard, ControlNode, Port, Edge, EdgeCount, NodeStatus, CanvasOutline, CanvasToolbar. Interaction is familiar to n8n users; the look is Orcon's own (owner decision Q6). No n8n code, icons, artwork or naming is used.

## Layout

1. Flows read left to right: triggers on the left, data flows to the right.
2. Snap grid: 8 px. Node positions are always multiples of 8 px.
3. Background: `canvas.bg` with a dotted grid in `canvas.grid` (decorative) aligned to the snap grid.
4. Minimap and zoom controls sit at the bottom right; CanvasToolbar sits at the top.
5. "Fit view" is always available from the toolbar and the keyboard.
6. Canvas positioning is the only place where inline `style` is allowed, and only for runtime coordinates and transforms ([tokens.md#usage-in-code](tokens.md#usage-in-code)).

## Nodes

Action and trigger nodes are cards (`NodeCard`):

1. Width: 240 px (comfortable) / 208 px (compact), from the `size.node-width` component token.
2. Header: `ConnectorIcon` + node name in `fg.default`, 14/20, weight 600.
3. Body: one-line summary of what the node does in `fg.muted`, truncated with a tooltip; the full summary is part of the accessible name.
4. Footer: `NodeStatus` (icon + text, [components.md#status](components.md#status)).
5. Radius 12 px (`lg`); fill `surface.default`; border `canvas.node.border` (decorative: the node is identified by its text); shadow sm in light, raised surface in dark.
6. Trigger nodes have no input port and show a leading start cap with a Lucide `Zap` badge.
7. Selection: a 2 px `canvas.selection` outline plus visible resize/drag handles. Selection is never shown by color alone: the selected node also exposes `aria-selected="true"` and appears in the selection count of the toolbar.
8. Hover shows the node actions (run step, disable, delete) as IconButtons; the same actions are reachable from the keyboard and the inspector.
9. Disabled nodes keep their text readable (`fg.muted`) and show a "Disabled" label; they are not only dimmed.

## Control nodes

Control nodes (if, switch, merge, loop, wait) have their own shape (`ControlNode`):

1. A smaller pill: height 40 px (comfortable) / 32 px (compact), full radius.
2. Content: icon + short name.
3. Outputs are labeled: "true" / "false" for if; the case names for switch; "done" / "loop" for loop. Labels come from the catalogs.
4. Control nodes use the same status, selection, focus and accessible-name rules as NodeCard.

## Ports and edges

1. Inputs on the left side of a node, outputs on the right side.
2. Port: 10 px circle in `canvas.port`; the hit area is enlarged to at least 24 px (44 px on coarse pointers).
3. Multiple outputs are stacked vertically and each one is labeled.
4. Edges: bezier curves, 2 px, `canvas.edge`. The active (executed) path uses `canvas.edge-active`: #16A34A in light (3.15:1 on `canvas.bg`), #22C55E in dark (8.26:1).
5. After an execution, each edge shows an `EdgeCount` chip on `surface.raised` with the item count ("12 items", ICU plural).
6. Loop-back edges are routed distinctly (below the nodes) and carry a chip with the Lucide `Repeat` icon.
7. Invalid connections (incompatible port types) are refused with a message that explains why; the canvas never infers parallelism from visual branching.
8. Edge color never carries meaning alone: the executed path also has the item count chip, and the outline lists it.

## Execution overlay

1. During and after an execution, every node shows its status per [components.md#status](components.md#status) (icon + text).
2. Nodes with UNCERTAIN items show an attention-family marker with the count ("2 uncertain", ICU plural) and link to the resolution panel.
3. Only the RUNNING indicator animates; with reduced motion it is static.
4. Status changes are announced through the polite, coalesced live region ([accessibility.md#live-regions](accessibility.md#live-regions)).
5. The overlay never hides the node name or summary.

## Keyboard

Every pointer interaction has a keyboard path:

| Key | Action |
|---|---|
| Tab | Enters the canvas (focus on the first node, then CanvasToolbar and the outline follow in order) |
| Arrow keys | Move focus spatially to the nearest node in that direction |
| Enter | Opens the inspector of the focused node |
| Shift + arrows | Move the selected node one grid step (8 px) |
| Alt + Shift + arrows | Move the selected node four grid steps (32 px) |
| A | Opens the add-node palette (searchable list) |
| C | Opens the Connect dialog: choose the source output and the target node and input from lists |
| Delete / Backspace | Removes the selection; multi-select asks for confirmation; an undo toast follows |
| Ctrl/Cmd + Z, Ctrl/Cmd + Y (or Shift + Z) | Undo, redo |
| Ctrl/Cmd + A | Select all nodes |
| Escape | Clears the selection or closes the open palette or dialog |
| ? | Lists all shortcuts (translatable labels) |

Dragging always has a single-pointer or keyboard alternative (WCAG 2.5.7): moving nodes with Shift + arrows, connecting with the Connect dialog.

## Screen reader

1. `CanvasOutline` is a panel that renders the graph as an accessible tree or list of nodes, connections and statuses. It is always reachable from the toolbar and the keyboard.
2. Node accessible name: "<kind> node <name>, <summary>, status <status>, <n> inputs, <m> outputs", built with ICU messages.
3. Edges are described in the outline as "<source> <output> to <target> <input>", with the item count after an execution.
4. Status changes are announced through the polite, coalesced live region; announcements never move focus.

## Mobile

1. Below md (768 px) the canvas is read-only: pan, zoom and tap.
2. Tapping a node opens a bottom sheet with its status, counts and a link to the execution detail.
3. No editing affordances are rendered below md: no add-node palette, no handles, no ports that accept connections, no delete actions.
4. CanvasOutline remains available on phones.

## Performance

1. Target: 60 fps pan and zoom with 300 nodes on a mid-range laptop. The editor specification measures it.
2. The add-node palette and long node lists are virtualized.
3. Edge geometry is not kept in per-frame React state; positions update through the canvas library.
4. No continuous animation of the graph; only the running indicator animates.
