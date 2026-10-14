---
title: The flow editor
description: Open a flow, change it on the canvas, copy and paste nodes, tidy it up and use the keyboard.
sources:
  - ADR-0007
  - ADR-0082
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# The flow editor

> This page describes the editor while it works on a sample flow kept in your browser tab. It does not read or save flows on the server yet and it does not run tests. The releases that deliver those features update this page.

The editor shows a flow as nodes joined by connections. You change the flow by adding nodes, moving them, joining them, renaming them and setting what each one does. Everything you can do with the pointer you can also do with the keyboard.

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

## Set what a node does

Press `Enter` on a focused node, or double-click it, to open its details. From a wide screen (1024 px and more) the details are a window over the canvas with three panes: **Input** on the left, the node's **Parameters** in the middle and **Output** on the right. A trigger has no input pane. On a tablet the parameters dock at the right of the canvas and the input and output data show in a panel under it, one side at a time. Press `Esc` or choose **Back to canvas** to close the details: focus goes back to the node. On a phone the details are not available; tapping a node shows its state.

The middle pane has three tabs:

- **Parameters** has one field for each thing the node needs. Each node type describes its own fields, so an HTTP Request has a method, a URL, headers, query parameters and a body, and an If has a condition. A field that holds text can be **Fixed** or an **Expression**: an expression is a formula, written between double curly braces, that is worked out for each item. A field that is required says so when you leave it empty. A field only shows while it applies, for example the body of an HTTP Request shows for methods that send one.
- **Settings** has the retries (how many attempts, how long to wait and how the wait grows), the time limits of one attempt and of the whole step, what happens when the step fails (stop, continue with the error as data, or send the failed items to a second **error** output), how many items the step handles at once, and whether people are told when a step that waits needs a decision. The text says what the default is: a step that only reads is retried three times, a step that changes something outside is not retried unless repeating it is known to be safe.
- **Docs** shows what the node type does, what each operation does, how its credentials work and what each parameter means.

Changes are written to the draft as you type and show on the canvas right away, for example the line under the node and the error output. Everything you type in one field while it keeps focus is one undo step.

### Credentials

A node that talks to another service has a **Credential** field listing the credentials of a matching type that you can use, with their type and when they changed. Secret values are never shown. **Create new credential** opens a form for the fields of the credential type without leaving the node; a secret field is write-only, and the new credential is chosen for the node when it is created. If the node names a credential the project does not have, the details say **Missing credential** and offer **Create** and **Choose another**; publishing fails until the name exists. Testing a credential needs the server, which the preview does not use.

### Call another flow

A **Call sub-flow** node runs another flow of the project. Choose the flow from the list, or give it by a formula and list the flows the formula may choose from. Choose the version (the current published one or a fixed number), whether to wait for the result or carry on, and whether to call once for each item. **Open sub-flow** goes to the called flow; `Ctrl` or `Cmd` and click opens it in a new tab. A flow that calls itself is warned about, and so is a flow that has no published version, because a test run calls the published one.

### New connector version

When a newer version of a node's connector is installed, the details say **New version available** with the version the node uses and the newest one. **What changed** lists the changes between them, and marks the ones that can alter what the step does so that you check them afterwards. **Update this node** changes the draft only, as one undo step; publishing stays a separate step.

Notes on a step and marks for sensitive values are not available yet.

## Undo and redo

`Ctrl` and `Z` undo, `Ctrl`, `Shift` and `Z` (or `Ctrl` and `Y`) redo. The editor keeps the last 100 changes. Each action is one step, including a paste and a tidy up.

## Copy, cut, paste and duplicate

`Ctrl` and `C` copy the selected nodes, the connections between them and the sticky notes as text in the `orcon/flow-fragment@1` format. You can paste that text in a forum or a pull request. It holds credentials by name only, never their values. `Ctrl` and `X` cut, `Ctrl` and `V` paste at the pointer and `Ctrl` and `D` duplicate. Pasted nodes get new ids and free names.

Pasting more than 2 MiB or more than 500 nodes at once is refused. Pasting an n8n workflow is recognized but not converted yet: conversion happens on the server.

## Saving

The editor saves the draft by itself two seconds after your last change, and at most ten seconds after the first unsaved change while you keep working. `Ctrl` and `S` save at once. The top bar shows **Saved**, **Saving…**, **Unsaved changes** or **Not saved, trying again**. If the draft changed somewhere else, nothing is overwritten: the editor says so and offers **Reload the flow**, which discards your unsaved changes after you confirm.

## Keyboard

Choose **Keyboard shortcuts** or press `?` for the full list. Single-key shortcuts work only while the canvas has focus, never while you type in a field. `Ctrl+K` opens the list of every command. **Flow outline** lists the nodes, their connections and the sticky notes as text, which is also how a screen reader reads the flow.
