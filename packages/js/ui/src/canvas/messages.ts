// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

export const canvasMessages = defineMessages({
  nodeLabel: {
    id: 'canvas.node.label',
    defaultMessage:
      '{kind, select, trigger {Trigger} control {Control} other {Action}} node {name}{hasSummary, select, yes {, {summary}} other {}}{hasStatus, select, yes {, status {status}} other {}}, {inputs, plural, one {# input} other {# inputs}}, {outputs, plural, one {# output} other {# outputs}}',
    description:
      'Accessible name of a node on the workflow canvas: its kind, name, one-line summary, execution status and how many inputs and outputs it has.',
  },
  nodeRoleDescription: {
    id: 'canvas.node.roleDescription',
    defaultMessage: 'node',
    description: 'Spoken role of a node on the workflow canvas, read after its name.',
  },
  uncertainItems: {
    id: 'canvas.node.uncertainItems',
    defaultMessage: '{count, plural, one {# uncertain item} other {# uncertain items}}',
    description: 'Footer of a node that has items whose external operation may or may not have been applied.',
  },
  startsFlow: {
    id: 'canvas.node.startsFlow',
    defaultMessage: 'Starts the flow',
    description: 'Accessible name of the badge on a trigger node. A trigger is what starts a flow.',
  },
  controlType: {
    id: 'canvas.control.type',
    defaultMessage: '{type, select, if {If} switch {Switch} merge {Merge} loop {Loop} wait {Wait} other {Control}}',
    description: 'Name of a kind of control node: a branch (if), a multi-way branch (switch), a merge, a loop or a wait.',
  },
  edgeItems: {
    id: 'canvas.edge.items',
    defaultMessage: '{count, plural, one {# item} other {# items}}',
    description: 'Chip on a connection that shows how many items travelled through it in the execution being shown.',
  },
  edgeLabel: {
    id: 'canvas.edge.label',
    defaultMessage:
      'Connection from {source} ({output}) to {target} ({input}){hasItems, select, yes {, {items, plural, one {# item} other {# items}}} other {}}{loop, select, yes {, loop back} other {}}',
    description: 'Accessible name of a connection between two nodes: where it starts, where it ends and how many items went through.',
  },
  outlineTitle: {
    id: 'canvas.outline.title',
    defaultMessage: 'Flow outline',
    description: 'Title of the panel that lists the nodes and connections of the flow as text, for screen readers and keyboard use.',
  },
  outlineSummary: {
    id: 'canvas.outline.summary',
    defaultMessage: '{nodes, plural, one {# node} other {# nodes}} and {edges, plural, one {# connection} other {# connections}}',
    description: 'Summary line of the flow outline: how many nodes and connections the flow has.',
  },
  outlineEmpty: {
    id: 'canvas.outline.empty',
    defaultMessage: 'This flow has no nodes yet.',
    description: 'Text of the flow outline when the flow has no nodes.',
  },
  outlineNoConnections: {
    id: 'canvas.outline.noConnections',
    defaultMessage: 'No connections to other nodes',
    description: 'Text under a node of the flow outline that is not connected to any other node.',
  },
  outlineConnection: {
    id: 'canvas.outline.connection',
    defaultMessage:
      '{direction, select, out {Output {port} goes to {other}, input {otherPort}} other {Input {port} receives from {other}, output {otherPort}}}{hasItems, select, yes {, {items, plural, one {# item} other {# items}}} other {}}',
    description:
      'One connection of a node in the flow outline. "out" means the node sends to another node; otherwise it receives from one.',
  },
  outlineFocus: {
    id: 'canvas.outline.focus',
    defaultMessage: 'Go to {name}',
    description: 'Button in the flow outline that moves focus to a node on the canvas. {name} is the node name.',
  },
  outlineDisconnect: {
    id: 'canvas.outline.disconnect',
    defaultMessage: 'Remove connection from {name} to {other}',
    description: 'Button in the flow outline that removes one connection. {name} is the node it starts at and {other} the node it goes to.',
  },
  outlineDelete: {
    id: 'canvas.outline.delete',
    defaultMessage: 'Delete {name}',
    description: 'Button in the flow outline that deletes a node. {name} is the node name.',
  },
  toolbarLabel: {
    id: 'canvas.toolbar.label',
    defaultMessage: 'Canvas tools',
    description: 'Accessible name of the toolbar above the workflow canvas.',
  },
  addNode: { id: 'canvas.toolbar.addNode', defaultMessage: 'Add node', description: 'Button that opens the list of nodes that can be added to the flow.' },
  connectNodes: {
    id: 'canvas.toolbar.connect',
    defaultMessage: 'Connect nodes',
    description: 'Button that opens a form to connect an output of one node to an input of another.',
  },
  deleteSelected: { id: 'canvas.toolbar.delete', defaultMessage: 'Delete selected', description: 'Button that deletes the selected nodes.' },
  undo: { id: 'canvas.toolbar.undo', defaultMessage: 'Undo', description: 'Button that undoes the last change to the flow.' },
  redo: { id: 'canvas.toolbar.redo', defaultMessage: 'Redo', description: 'Button that redoes the last undone change to the flow.' },
  zoomIn: { id: 'canvas.toolbar.zoomIn', defaultMessage: 'Zoom in', description: 'Button that makes the canvas larger.' },
  zoomOut: { id: 'canvas.toolbar.zoomOut', defaultMessage: 'Zoom out', description: 'Button that makes the canvas smaller.' },
  fitView: { id: 'canvas.toolbar.fit', defaultMessage: 'Fit flow to screen', description: 'Button that zooms the canvas so the whole flow is visible.' },
  showOutline: {
    id: 'canvas.toolbar.outline',
    defaultMessage: 'Flow outline',
    description: 'Button that shows or hides the text outline of the flow.',
  },
  shortcuts: {
    id: 'canvas.toolbar.shortcuts',
    defaultMessage: 'Keyboard shortcuts',
    description: 'Button that opens the list of keyboard shortcuts of the canvas.',
  },
  readOnly: {
    id: 'canvas.readOnly',
    defaultMessage: 'View only. Open the flow on a tablet or a computer to edit it.',
    description: 'Notice on the canvas on a phone, where flows can be looked at but not edited.',
  },
  canvasLabel: {
    id: 'canvas.label',
    defaultMessage: 'Flow canvas: {nodes, plural, one {# node} other {# nodes}}',
    description: 'Accessible name of the workflow canvas region. {nodes} is the number of nodes.',
  },
  emptyTitle: { id: 'canvas.empty.title', defaultMessage: 'This flow is empty', description: 'Title shown on the canvas when the flow has no nodes.' },
  emptyDescription: {
    id: 'canvas.empty.description',
    defaultMessage: 'Add a trigger to decide when the flow starts.',
    description: 'Text shown on the canvas when the flow has no nodes, saying what to do next.',
  },
  paletteTitle: { id: 'canvas.palette.title', defaultMessage: 'Add a node', description: 'Title of the searchable list of nodes that can be added.' },
  paletteSearch: {
    id: 'canvas.palette.search',
    defaultMessage: 'Search nodes',
    description: 'Accessible name and placeholder of the search box in the add-node list.',
  },
  paletteEmpty: {
    id: 'canvas.palette.empty',
    defaultMessage: 'No node matches your search',
    description: 'Shown in the add-node list when nothing matches what was typed.',
  },
  connectTitle: { id: 'canvas.connect.title', defaultMessage: 'Connect nodes', description: 'Title of the form that connects two nodes.' },
  connectDescription: {
    id: 'canvas.connect.description',
    defaultMessage: 'Choose where the data leaves and where it goes. The connection is drawn on the canvas.',
    description: 'Explanation in the form that connects two nodes.',
  },
  connectSource: { id: 'canvas.connect.source', defaultMessage: 'From node', description: 'Label of the field that picks the node a connection starts at.' },
  connectOutput: { id: 'canvas.connect.output', defaultMessage: 'Output', description: 'Label of the field that picks the output of the starting node.' },
  connectTarget: { id: 'canvas.connect.target', defaultMessage: 'To node', description: 'Label of the field that picks the node a connection ends at.' },
  connectInput: { id: 'canvas.connect.input', defaultMessage: 'Input', description: 'Label of the field that picks the input of the ending node.' },
  connectChoose: { id: 'canvas.connect.choose', defaultMessage: 'Choose', description: 'Placeholder of a field in the connect form where nothing is chosen yet.' },
  connectSubmit: { id: 'canvas.connect.submit', defaultMessage: 'Connect', description: 'Button that creates the connection.' },
  connectErrorSame: {
    id: 'canvas.connect.error.same',
    defaultMessage: 'A node cannot be connected to itself.',
    description: 'Error in the connect form when the same node is chosen at both ends.',
  },
  connectErrorDuplicate: {
    id: 'canvas.connect.error.duplicate',
    defaultMessage: 'These two ports are already connected.',
    description: 'Error in the connect form when the connection already exists.',
  },
  connectErrorTrigger: {
    id: 'canvas.connect.error.trigger',
    defaultMessage: 'A trigger starts the flow, so nothing can be connected into it.',
    description: 'Error in the connect form when the target is a trigger.',
  },
  connectErrorIncomplete: {
    id: 'canvas.connect.error.incomplete',
    defaultMessage: 'Choose both nodes and both ports.',
    description: 'Error in the connect form when a field is empty.',
  },
  deleteMany: {
    id: 'canvas.delete.title',
    defaultMessage: '{count, plural, one {Delete # node?} other {Delete # nodes?}}',
    description: 'Question of the confirmation before deleting several nodes.',
  },
  deleteManyImpact: {
    id: 'canvas.delete.impact',
    defaultMessage: 'The nodes and their connections leave the flow. You can undo this right after.',
    description: 'Explains what deleting nodes does and that it can be undone.',
  },
  deleteConfirm: { id: 'canvas.delete.confirm', defaultMessage: 'Delete nodes', description: 'Button that confirms deleting the selected nodes.' },
  shortcutsTitle: { id: 'canvas.shortcuts.title', defaultMessage: 'Keyboard shortcuts', description: 'Title of the list of canvas keyboard shortcuts.' },
  shortcutsMove: {
    id: 'canvas.shortcuts.move',
    defaultMessage: 'Move focus between nodes',
    description: 'Shortcut description: arrow keys move focus to the nearest node.',
  },
  shortcutsOpen: { id: 'canvas.shortcuts.open', defaultMessage: 'Open the node details', description: 'Shortcut description: Enter opens the node inspector.' },
  shortcutsSelect: {
    id: 'canvas.shortcuts.select',
    defaultMessage: 'Select or unselect the focused node',
    description: 'Shortcut description: Space toggles the selection of the focused node.',
  },
  shortcutsNudge: {
    id: 'canvas.shortcuts.nudge',
    defaultMessage: 'Move the selected node one grid step',
    description: 'Shortcut description: Shift plus arrow moves the node by one step of the grid.',
  },
  shortcutsNudgeFar: {
    id: 'canvas.shortcuts.nudgeFar',
    defaultMessage: 'Move the selected node four grid steps',
    description: 'Shortcut description: Alt and Shift plus arrow moves the node by four steps of the grid.',
  },
  shortcutsAdd: { id: 'canvas.shortcuts.add', defaultMessage: 'Add a node', description: 'Shortcut description: A opens the add-node list.' },
  shortcutsConnect: { id: 'canvas.shortcuts.connect', defaultMessage: 'Connect two nodes', description: 'Shortcut description: C opens the connect form.' },
  shortcutsDelete: {
    id: 'canvas.shortcuts.delete',
    defaultMessage: 'Delete the selected nodes',
    description: 'Shortcut description: Delete removes the selected nodes.',
  },
  shortcutsUndo: { id: 'canvas.shortcuts.undo', defaultMessage: 'Undo', description: 'Shortcut description: undo the last change.' },
  shortcutsRedo: { id: 'canvas.shortcuts.redo', defaultMessage: 'Redo', description: 'Shortcut description: redo the last undone change.' },
  shortcutsHelp: { id: 'canvas.shortcuts.help', defaultMessage: 'Show this list', description: 'Shortcut description: the question mark opens this list.' },
  shortcutsClear: { id: 'canvas.shortcuts.clear', defaultMessage: 'Clear the selection', description: 'Shortcut description: Escape unselects all nodes.' },
  announceAdded: { id: 'canvas.announce.added', defaultMessage: 'Added {name}', description: 'Spoken after a node was added. {name} is the node name.' },
  announceMoved: {
    id: 'canvas.announce.moved',
    defaultMessage: 'Moved {name} to column {column}, row {row}',
    description: 'Spoken after a node was moved with the keyboard. Column and row are grid positions.',
  },
  announceDeleted: {
    id: 'canvas.announce.deleted',
    defaultMessage: '{count, plural, one {Deleted # node} other {Deleted # nodes}}',
    description: 'Spoken after nodes were deleted.',
  },
  announceConnected: {
    id: 'canvas.announce.connected',
    defaultMessage: 'Connected {source} to {target}',
    description: 'Spoken after two nodes were connected.',
  },
  announceDisconnected: {
    id: 'canvas.announce.disconnected',
    defaultMessage: 'Removed the connection from {source} to {target}',
    description: 'Spoken after a connection was removed.',
  },
  announceUndone: { id: 'canvas.announce.undone', defaultMessage: 'Undid the last change', description: 'Spoken after an undo.' },
  announceRedone: { id: 'canvas.announce.redone', defaultMessage: 'Redid the change', description: 'Spoken after a redo.' },
  announceStatus: {
    id: 'canvas.announce.status',
    defaultMessage: '{name} is now {status}',
    description: 'Spoken when the execution state of a node changes. {status} is the state label.',
  },
  undoToast: {
    id: 'canvas.toast.deleted',
    defaultMessage: '{count, plural, one {Deleted # node} other {Deleted # nodes}}',
    description: 'Message of the short notice shown after nodes were deleted, next to an Undo button.',
  },
  undoAction: { id: 'canvas.toast.undo', defaultMessage: 'Undo', description: 'Button inside the notice that brings back deleted nodes.' },
  detailsStatus: { id: 'canvas.details.status', defaultMessage: 'Status', description: 'Label of the execution state in the node details.' },
  detailsReceived: {
    id: 'canvas.details.received',
    defaultMessage: '{count, plural, one {# item received} other {# items received}}',
    description: 'Number of items that arrived at the node in the execution being shown.',
  },
  detailsSent: {
    id: 'canvas.details.sent',
    defaultMessage: '{count, plural, one {# item sent on} other {# items sent on}}',
    description: 'Number of items that left the node in the execution being shown.',
  },
  detailsExecution: {
    id: 'canvas.details.execution',
    defaultMessage: 'Open the execution detail',
    description: 'Link in the node details that opens the execution detail page.',
  },
  detailsNoStatus: {
    id: 'canvas.details.noStatus',
    defaultMessage: 'This node has not run in the execution being shown.',
    description: 'Text in the node details when there is no execution state for the node.',
  },
  ariaNodeDescription: {
    id: 'canvas.aria.nodeDescription',
    defaultMessage: 'Use the arrow keys to move to another node. Press Enter to open its details.',
    description: 'Hint read by screen readers for a node on the canvas.',
  },
  ariaControls: { id: 'canvas.aria.controls', defaultMessage: 'Zoom controls', description: 'Accessible name of the group of zoom buttons.' },
  ariaMinimap: { id: 'canvas.aria.minimap', defaultMessage: 'Overview map of the flow', description: 'Accessible name of the small map that shows the whole flow.' },
});
