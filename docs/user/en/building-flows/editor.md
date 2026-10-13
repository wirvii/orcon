---
title: The flow editor
description: Open a flow, change it on the canvas, copy and paste nodes, tidy it up and use the keyboard.
sources:
  - ADR-0007
  - ADR-0082
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# The flow editor

> This page describes the first release of the editor, which works on a sample flow kept in your browser tab. It does not read or save flows on the server yet, it does not edit what a node does (its parameters) and it does not run tests. The slices that deliver those features update this page.

The editor shows a flow as nodes joined by connections. You change the flow by adding nodes, moving them, joining them and renaming them. Everything you can do with the pointer you can also do with the keyboard.

## Open the editor

Choose **Flow editor** in the navigation. It opens a sample flow at `/flows/01930000-0000-7000-8000-00000000ed5a`: a webhook, an If node that checks an order, two HTTP requests and a sticky note.

The page says **Preview with a sample flow**. Your changes stay in the browser tab and are lost when you reload.

On a phone the flow is shown read-only: you can pan, zoom and tap a node to see its state. Editing needs a tablet or a computer.

## Change the flow

| To | Do this |
|---|---|
| Add a node | Choose **Add node**, or press `A`, then pick a node. A new node lands next to the focused node and is connected to it. |
| Move nodes | Drag them, or press `Shift` with the arrow keys (one grid step of 20 px) or `Alt` and `Shift` with the arrow keys (four steps). |
| Join two nodes | Drag from an output port to another node, or press `C`. A connection cannot go into a trigger, and a cycle is only allowed back to a Loop node. |
| Rename a node | Press `F2`. Expressions that read the node by its name (`$('Old name')`) are updated and the editor tells you how many. |
| Disable or enable a node | Press `D`. |
| Add a sticky note | Choose **Add sticky note**, or press `Shift` and `S`. |
| Delete | Press `Delete`. Deleting several nodes asks first and offers to undo. |
| Find a node | Press `Ctrl` and `F` (`Cmd` and `F` on a Mac). |
| Tidy up | Choose **Tidy up**, or press `Shift`, `Alt` and `T`. With two or more nodes selected only those are arranged; otherwise the whole flow is. |

Nodes sit on a 20 px grid. Names are unique in a flow: when you add a node whose name is taken, the editor numbers it ("HTTP Request 2").

## Undo and redo

`Ctrl` and `Z` undo, `Ctrl`, `Shift` and `Z` (or `Ctrl` and `Y`) redo. The editor keeps the last 100 changes. Each action is one step, including a paste and a tidy up.

## Copy, cut, paste and duplicate

`Ctrl` and `C` copy the selected nodes, the connections between them and the sticky notes as text in the `orcon/flow-fragment@1` format. You can paste that text in a forum or a pull request. It holds credentials by name only, never their values. `Ctrl` and `X` cut, `Ctrl` and `V` paste at the pointer and `Ctrl` and `D` duplicate. Pasted nodes get new ids and free names.

Pasting more than 2 MiB or more than 500 nodes at once is refused. Pasting an n8n workflow is recognized but not converted yet: conversion happens on the server.

## Saving

The editor saves the draft by itself two seconds after your last change, and at most ten seconds after the first unsaved change while you keep working. `Ctrl` and `S` save at once. The top bar shows **Saved**, **Saving…**, **Unsaved changes** or **Not saved, trying again**. If the draft changed somewhere else, nothing is overwritten: the editor says so and offers **Reload the flow**, which discards your unsaved changes after you confirm.

## Keyboard

Choose **Keyboard shortcuts** or press `?` for the full list. Single-key shortcuts work only while the canvas has focus, never while you type in a field. `Ctrl+K` opens the list of every command. **Flow outline** lists the nodes, their connections and the sticky notes as text, which is also how a screen reader reads the flow.
