<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Flow editor keyboard reference

Status: Proposed — pending owner approval

Single-key shortcuts work only while the canvas has focus, never inside text fields. Ctrl means Cmd on macOS. The in-app `?` dialog shows the same list in the user's language.

| Keys | Action | Scope |
|---|---|---|
| Tab / Shift+Tab | Move between toolbar, canvas and panels; Tab enters the canvas | Editor |
| Arrow keys | Move focus to the nearest node in that direction; with no node focused, pan | Canvas |
| Enter | Open the node details view for the focused node | Canvas |
| Esc | Close the open panel or dialog; clear selection | Editor |
| Shift+Arrow / Alt+Shift+Arrow | Move selected nodes 1 / 4 grid steps (20 / 80 px) | Canvas |
| A | Add a node (palette) after the focused node or on the focused edge | Canvas |
| C | Connect: choose source output and target from lists | Canvas |
| Delete / Backspace | Delete the selection (nodes or a connection) | Canvas |
| F2 | Rename the focused node (updates references in expressions) | Canvas |
| D | Disable or enable the focused node | Canvas |
| P | Pin or unpin the focused node's output | Canvas |
| Shift+S | Add a sticky note | Canvas |
| Shift+Alt+T | Tidy up (auto-layout) the selection or the flow | Canvas |
| Ctrl+A | Select all | Canvas |
| Ctrl+C / Ctrl+X / Ctrl+V | Copy / cut / paste nodes (Orcon or n8n JSON) | Canvas |
| Ctrl+D | Duplicate the selection | Canvas |
| Ctrl+Z / Ctrl+Shift+Z or Ctrl+Y | Undo / redo | Editor |
| Ctrl+S | Save the draft now | Editor |
| Ctrl+Enter | Test: run the flow | Editor |
| Ctrl+Shift+Enter | Test: run up to the focused node | Canvas |
| R | Test: run only the focused node | Canvas |
| Ctrl+F | Find a node by name | Editor |
| 1 / 0 / + / - | Zoom to fit / 100 % / in / out | Canvas |
| O | Show or hide the outline (accessible list of the graph) | Canvas |
| Ctrl+K | Command palette (every editor command, including publish and versions) | Editor |
| ? | Show keyboard shortcuts | Canvas |
| Ctrl+Space | Open autocomplete | Expression field |
| Alt+Enter | Open the large expression editor | Expression field |
| Enter (on a field in the input panel) | Insert that field into the last focused expression | Data panel |
| Ctrl+Shift+C (on a field) | Copy the field path | Data panel |
| N / Shift+N | Next / previous change (compare) or next / previous node in run order (debugger) | Compare, debugger |
