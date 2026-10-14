// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { toCanvasGraph, type CanvasGraph } from '../model/canvas-graph';
import { exampleFlowDocument } from '../model/example-flow';
import { cloneDocument, type FlowDocument } from '../model/flow-document';

import { createEditorStore, HISTORY_LIMIT, type EditorState, type EditorStore } from './editor-store';

const open = () => createEditorStore({ document: exampleFlowDocument(), revision: '1' });
const snapshot = (store: EditorStore): FlowDocument => cloneDocument(store.getState().document);

/**
 * ST-7: for an action, applying it and undoing it returns a document deep-equal to the one before,
 * and undoing then redoing returns the document the action made.
 */
function expectUndoable(act: (state: EditorState) => unknown, options: { changes?: boolean } = {}) {
  const store = open();
  const before = snapshot(store);
  act(store.getState());
  const after = snapshot(store);
  if (options.changes !== false) expect(after).not.toEqual(before);
  expect(store.getState().history.past).toHaveLength(options.changes === false ? 0 : 1);
  expect(store.getState().undo()).toBe(options.changes !== false);
  expect(store.getState().document).toEqual(before);
  expect(store.getState().redo()).toBe(options.changes !== false);
  expect(store.getState().document).toEqual(after);
}

describe('the editor store: every action can be undone and redone', () => {
  it('addNode', () => expectUndoable((state) => state.addNode('http', { x: 500, y: 300 }, 'HTTP Request')));
  it('removeNodes', () => expectUndoable((state) => state.removeNodes(['inventory', 'note-1'])));
  it('connect', () => expectUndoable((state) => state.connect('inventory', 'main', 'rejected')));
  it('disconnect', () => expectUndoable((state) => state.disconnect('e3')));
  it('moveNodes', () => expectUndoable((state) => state.moveNodes(['slack', 'note-1'], 40, 20)));
  it('renameNode, with the references it rewrites', () => expectUndoable((state) => state.renameNode('webhook', 'Order received')));
  it('setDisabled', () => expectUndoable((state) => state.setDisabled('slack', true)));
  it('addNote', () => expectUndoable((state) => state.addNote({ x: 100, y: 500 })));
  it('setParameter', () => expectUndoable((state) => state.setParameter('slack', 'method', 'PUT')));
  it('setNodeBlock', () => expectUndoable((state) => state.setNodeBlock('slack', 'retry', { maxAttempts: 4, initialInterval: 'PT2S' })));
  it('setCredential', () => expectUndoable((state) => state.setCredential('slack', 'auth', 'Inventory API key')));
  it('updateConnector', () => expectUndoable((state) => state.updateConnector('slack', '1.2.0', 'sha256:c3')));
  it('updateNote', () => expectUndoable((state) => state.updateNote('note-1', { body: 'changed', color: 4 })));
  it('applyLayout', () => expectUndoable((state) => state.applyLayout(new Map([['slack', { x: 700, y: 120 }]]))));
  it('duplicate', () => expectUndoable((state) => state.duplicate(['inventory', 'slack'])));
  it('paste', () =>
    expectUndoable((state) => {
      state.paste({ format: 'orcon/flow-fragment@1', nodes: [{ id: 'x', name: 'X', element: 'task', type: 'orcon.noop@1', parameters: {} }], edges: [], editor: {} }, { at: { x: 60, y: 60 } });
    }));
  it('applyCanvasEdit', () =>
    expectUndoable((state) => {
      const graph = toCanvasGraph(state.document);
      const moved: CanvasGraph = { ...graph, nodes: graph.nodes.map((node) => (node.id === 'done' ? { ...node, position: { x: 900, y: 100 } } : node)) };
      state.applyCanvasEdit(graph, moved);
    }));

  it('refuses an action that changes nothing without recording an entry', () => {
    const store = open();
    expect(store.getState().removeNodes(['ghost'])).toEqual([]);
    expect(store.getState().disconnect('ghost')).toBe(false);
    expect(store.getState().addNode('nope', { x: 0, y: 0 }, 'x')).toBeUndefined();
    expect(store.getState().renameNode('ghost', 'x')).toEqual({ problem: 'missing' });
    expect(store.getState().connect('slack', 'main', 'validate')).toEqual({ problem: 'cycle' });
    expect(store.getState().history.past).toHaveLength(0);
    expect(store.getState().version).toBe(0);
    expect(store.getState().undo()).toBe(false);
    expect(store.getState().redo()).toBe(false);
  });
});

describe('the editor store: what actions return', () => {
  it('returns the id, the edge, the references and the pasted ids', () => {
    const store = open();
    expect(store.getState().addNode('if', { x: 0, y: 0 }, 'If')).toBe('add-if-1');
    expect(store.getState().connect('inventory', 'main', 'rejected')).toEqual({ edgeId: 'edge-inventory-rejected-1' });
    expect(store.getState().renameNode('webhook', 'Order received')).toEqual({ references: 1 });
    expect(store.getState().addNote({ x: 0, y: 0 })).toBe('note-2');
    expect(store.getState().removeNodes(['done'])).toEqual(['done']);
    expect(store.getState().duplicate(['slack'])).toEqual({ nodeIds: ['slack-1'], noteIds: [], droppedEdges: 0, placeholders: 0 });
  });
});

describe('the editor store: history', () => {
  it('records one entry per action and restores the selection that went with it', () => {
    const store = open();
    store.getState().select(['slack']);
    store.getState().addNode('end', { x: 0, y: 0 }, 'End');
    expect(store.getState().selection).toEqual(['add-end-1']);
    store.getState().undo();
    expect(store.getState().selection).toEqual(['slack']);
    store.getState().redo();
    expect(store.getState().selection).toEqual(['add-end-1']);
  });

  it('selects what a paste added and what a delete leaves: nothing', () => {
    const store = open();
    store.getState().duplicate(['slack', 'note-1']);
    expect(store.getState().selection).toEqual(['slack-1', 'note-2']);
    store.getState().removeNodes(['slack-1']);
    expect(store.getState().selection).toEqual([]);
  });

  it('a new action drops what could have been redone', () => {
    const store = open();
    store.getState().setDisabled('slack', true);
    store.getState().undo();
    expect(store.getState().history.future).toHaveLength(1);
    store.getState().setDisabled('done', true);
    expect(store.getState().history.future).toHaveLength(0);
  });

  it('keeps the last 100 entries', () => {
    const store = open();
    for (let step = 0; step < HISTORY_LIMIT + 20; step += 1) store.getState().moveNodes(['slack'], 20, 0);
    expect(store.getState().history.past).toHaveLength(HISTORY_LIMIT);
    for (let step = 0; step < HISTORY_LIMIT; step += 1) store.getState().undo();
    expect(store.getState().undo()).toBe(false);
    expect(store.getState().document.editor.nodes?.['slack']).toEqual({ x: 640 + 20 * 20, y: 60 });
  });

  it('starts again when the document is replaced by one loaded from the server', () => {
    const store = open();
    store.getState().setDisabled('slack', true);
    const fresh = exampleFlowDocument();
    fresh.name = 'Reloaded';
    store.getState().replaceDocument(fresh, '7');
    expect(store.getState()).toMatchObject({ baseRevision: '7', version: 0, savedVersion: 0, selection: [] });
    expect(store.getState().history).toEqual({ past: [], future: [] });
    expect(store.getState().document.name).toBe('Reloaded');
  });
});

describe('the editor store: the canvas', () => {
  it('asks for the canvas to be rebuilt after an action the canvas did not make', () => {
    const store = open();
    expect(store.getState().canvasEpoch).toBe(0);
    store.getState().applyLayout(new Map([['slack', { x: 700, y: 120 }]]));
    expect(store.getState().canvasEpoch).toBe(1);
    store.getState().undo();
    expect(store.getState().canvasEpoch).toBe(2);
    store.getState().rebuildCanvas();
    expect(store.getState().canvasEpoch).toBe(3);
  });

  it('does not rebuild it after a change the canvas made, nor when the canvas undoes it itself', () => {
    const store = open();
    const graph = toCanvasGraph(store.getState().document);
    const moved: CanvasGraph = { ...graph, nodes: graph.nodes.map((node) => (node.id === 'done' ? { ...node, position: { x: 900, y: 100 } } : node)) };
    expect(store.getState().applyCanvasEdit(graph, moved).divergent).toBe(false);
    expect(store.getState().canvasEpoch).toBe(0);
    store.getState().undo({ fromCanvas: true });
    expect(store.getState().canvasEpoch).toBe(0);
    store.getState().redo({ fromCanvas: true });
    expect(store.getState().canvasEpoch).toBe(0);
    // An entry the canvas did not make is never undone behind its back.
    store.getState().setDisabled('slack', true);
    store.getState().undo({ fromCanvas: true });
    expect(store.getState().canvasEpoch).toBe(2);
  });

  it('rebuilds it when the document refused part of what the canvas did, and keeps the rest', () => {
    const store = open();
    const graph = toCanvasGraph(store.getState().document);
    const next: CanvasGraph = {
      ...graph,
      nodes: graph.nodes.map((node) => (node.id === 'done' ? { ...node, position: { x: 900, y: 100 }, pinned: true } : node)),
    };
    const report = store.getState().applyCanvasEdit(graph, next);
    expect(report).toMatchObject({ divergent: true, pinIgnored: ['done'] });
    expect(store.getState().canvasEpoch).toBe(1);
    expect(store.getState().document.editor.nodes?.['done']).toEqual({ x: 900, y: 100 });
  });

  it('rebuilds it when nothing could be applied but the canvas shows something else', () => {
    const store = open();
    const graph = toCanvasGraph(store.getState().document);
    const next: CanvasGraph = { ...graph, nodes: graph.nodes.map((node) => (node.id === 'done' ? { ...node, pinned: true } : node)) };
    store.getState().applyCanvasEdit(graph, next);
    expect(store.getState().canvasEpoch).toBe(1);
    expect(store.getState().history.past).toHaveLength(0);
  });
});

describe('the editor store: saving', () => {
  it('counts changes so autosave can tell what is saved', () => {
    const store = open();
    store.getState().setDisabled('slack', true);
    store.getState().setDisabled('done', true);
    expect(store.getState()).toMatchObject({ version: 2, savedVersion: 0 });
    store.getState().markSaved(2, '2');
    expect(store.getState()).toMatchObject({ savedVersion: 2, baseRevision: '2' });
    store.getState().undo();
    expect(store.getState().version).toBe(3);
  });

  it('keeps the last text copied for when the system clipboard refuses', () => {
    const store = open();
    store.getState().setClipboardFallback('{"format":"orcon/flow-fragment@1"}');
    expect(store.getState().clipboardFallback).toContain('flow-fragment');
    store.getState().setClipboardFallback(null);
    expect(store.getState().clipboardFallback).toBeNull();
  });

  it('is one store per editor: two stores do not share a document', () => {
    const [first, second] = [open(), open()];
    first.getState().setDisabled('slack', true);
    expect(second.getState().document.nodes.find((node) => node.id === 'slack')).not.toHaveProperty('disabled');
  });
});

describe('the editor store: configuring a node', () => {
  const urlOf = (store: EditorStore) => store.getState().document.nodes.find((node) => node.id === 'slack')?.parameters['url'];

  it('makes one history entry of the edits made in one field while it kept its group', () => {
    const store = open();
    const original = urlOf(store);
    for (const text of ['h', 'ht', 'htt', 'http']) store.getState().setParameter('slack', 'url', text, 'url#1');
    expect(urlOf(store)).toBe('http');
    expect(store.getState().history.past).toHaveLength(1);
    expect(store.getState().history.past[0]?.name).toBe('configureNode');
    store.getState().undo();
    expect(urlOf(store)).toBe(original);
    store.getState().redo();
    expect(urlOf(store)).toBe('http');
  });

  it('starts a new entry when the group changes, as it does when the field is focused again', () => {
    const store = open();
    store.getState().setParameter('slack', 'url', 'a', 'url#1');
    store.getState().setParameter('slack', 'url', 'ab', 'url#1');
    store.getState().setParameter('slack', 'url', 'abc', 'url#2');
    expect(store.getState().history.past).toHaveLength(2);
    store.getState().undo();
    expect(urlOf(store)).toBe('ab');
  });

  it('does not join an edit to the entry of another field, even with the same group', () => {
    const store = open();
    store.getState().setParameter('slack', 'url', 'a', 'g');
    store.getState().setNodeBlock('slack', 'retry', { maxAttempts: 2 });
    store.getState().setParameter('slack', 'url', 'ab', 'g');
    expect(store.getState().history.past).toHaveLength(3);
  });

  it('does not join an edit to an entry that undo or redo has passed through', () => {
    const store = open();
    store.getState().setParameter('slack', 'url', 'a', 'g');
    store.getState().undo();
    store.getState().redo();
    store.getState().setParameter('slack', 'url', 'ab', 'g');
    expect(store.getState().history.past).toHaveLength(2);
  });

  it('keeps edits without a group as separate entries', () => {
    const store = open();
    store.getState().setParameter('slack', 'method', 'PUT');
    store.getState().setParameter('slack', 'method', 'PATCH');
    expect(store.getState().history.past).toHaveLength(2);
  });

  it('records nothing when the value does not change', () => {
    const store = open();
    store.getState().setParameter('slack', 'method', 'POST');
    store.getState().setCredential('slack', 'auth', 'Slack incoming webhook');
    expect(store.getState().history.past).toHaveLength(0);
    expect(store.getState().version).toBe(0);
  });

  it('does not rebuild the canvas for an edit of the form, and rebuilds it when the edit is taken back', () => {
    const store = open();
    store.getState().setParameter('slack', 'method', 'PUT');
    expect(store.getState().canvasEpoch).toBe(0);
    store.getState().undo();
    expect(store.getState().canvasEpoch).toBe(1);
  });

  it('is one history entry to update the connector, and undo gives the old pin back', () => {
    const store = open();
    store.getState().updateConnector('slack', '1.2.0', 'sha256:c3');
    expect(store.getState().history.past).toHaveLength(1);
    expect(store.getState().history.past[0]?.name).toBe('updateConnector');
    store.getState().undo();
    expect(store.getState().document.nodes.find((node) => node.id === 'slack')?.['connector']).toMatchObject({ version: '1.0.0' });
  });

  it('shows the canvas the ports a failure output adds', () => {
    const store = open();
    store.getState().setNodeBlock('slack', 'onError', 'ERROR_OUTPUT');
    const graph: CanvasGraph = toCanvasGraph(store.getState().document);
    expect(graph.nodes.find((node) => node.id === 'slack')?.outputs.map((port) => port.id)).toEqual(['main', 'error']);
  });
});
