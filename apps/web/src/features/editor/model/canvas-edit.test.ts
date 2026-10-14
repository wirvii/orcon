// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { applyCanvasEdit } from './canvas-edit';
import { toCanvasGraph, graphAgrees, type CanvasGraph, type CanvasNode } from './canvas-graph';
import { exampleFlowDocument } from './example-flow';
import { findNode } from './flow-document';

/** What the canvas does on a change: a new graph that shares every untouched node with the old one. */
function change(graph: CanvasGraph, update: (draft: CanvasGraph) => void): CanvasGraph {
  const next: CanvasGraph = { nodes: [...graph.nodes], edges: [...graph.edges], notes: [...(graph.notes ?? [])] };
  update(next);
  return next;
}

const replaceNode = (graph: CanvasGraph, id: string, patch: Partial<CanvasNode>) => {
  const index = graph.nodes.findIndex((node) => node.id === id);
  const old = graph.nodes[index];
  if (!old) throw new Error(`no node ${id}`);
  graph.nodes[index] = { ...old, ...patch };
};

const tile = (id: string, name: string, extra: Partial<CanvasNode> = {}): CanvasNode => ({
  id,
  kind: 'action',
  name,
  inputs: [{ id: 'main' }],
  outputs: [{ id: 'main' }],
  position: { x: 640, y: 260 },
  ...extra,
});

function apply(update: (draft: CanvasGraph) => void) {
  const document = exampleFlowDocument();
  const before = toCanvasGraph(document);
  const after = change(before, update);
  const report = applyCanvasEdit(document, before, after);
  return { document, before, after, report };
}

describe('applying what the canvas changed', () => {
  it('records a moved node and nothing else', () => {
    const { document, report, after } = apply((graph) => replaceNode(graph, 'slack', { position: { x: 680, y: 100 } }));
    expect(document.editor.nodes?.['slack']).toEqual({ x: 680, y: 100 });
    expect(report.divergent).toBe(false);
    expect(graphAgrees(document, after)).toBe(true);
    expect(document.nodes).toEqual(exampleFlowDocument().nodes);
  });

  it('adds a node from the palette with the type, blocks and position of its entry', () => {
    const { document, report } = apply((graph) => {
      graph.nodes.push(tile('add-if-1', 'If', { kind: 'logic', logicType: 'if', outputs: [{ id: 'true' }, { id: 'false' }] }));
    });
    expect(findNode(document, 'add-if-1')).toMatchObject({ name: 'If', type: 'orcon.if@1', element: 'exclusiveGateway', routing: 'EXCLUSIVE', parameters: { condition: '' } });
    expect(document.editor.nodes?.['add-if-1']).toEqual({ x: 640, y: 260 });
    expect(report.divergent).toBe(false);
  });

  it('gives a node added from the palette a free name when its own is taken, and says so', () => {
    const { document, report } = apply((graph) => {
      graph.nodes.push(tile('add-http-1', 'HTTP Request'));
      graph.nodes.push(tile('add-http-2', 'HTTP Request', { position: { x: 640, y: 400 } }));
    });
    expect(['add-http-1', 'add-http-2'].map((id) => findNode(document, id)?.name)).toEqual(['HTTP Request 2', 'HTTP Request 3']);
    expect(report.adjustedNames).toEqual([
      { id: 'add-http-1', name: 'HTTP Request 2' },
      { id: 'add-http-2', name: 'HTTP Request 3' },
    ]);
    expect(report.divergent).toBe(false);
  });

  it('makes a copy of a node when the canvas duplicates it, with its parameters and a free name', () => {
    const { document, report } = apply((graph) => {
      graph.nodes.push(tile('inventory-1', 'HTTP Request', { position: { x: 440, y: 220 } }));
    });
    expect(findNode(document, 'inventory-1')).toMatchObject({ type: 'orcon.http@1', name: 'HTTP Request 2', parameters: { method: 'GET', url: '=https://inventory.example.test/items/{{ $json.sku }}' } });
    // The canvas still shows the old name: it is told the free one, and nothing needs rebuilding.
    expect(report.adjustedNames).toEqual([{ id: 'inventory-1', name: 'HTTP Request 2' }]);
    expect(report.divergent).toBe(false);
  });

  it('falls back to a plain node for an id it does not know', () => {
    const { document } = apply((graph) => {
      graph.nodes.push(tile('mystery', 'Mystery'));
    });
    expect(findNode(document, 'mystery')?.type).toBe('orcon.noop@1');
  });

  it('renames a node and rewrites the references to it', () => {
    const { document, report } = apply((graph) => replaceNode(graph, 'webhook', { name: 'Order received' }));
    expect(report.renamed).toEqual([{ id: 'webhook', from: 'Webhook', to: 'Order received', references: 1 }]);
    expect(String(findNode(document, 'slack')?.parameters['body'])).toContain(`$('Order received')`);
  });

  it('refuses a name that is taken or empty and asks for a rebuild', () => {
    expect(apply((graph) => replaceNode(graph, 'webhook', { name: 'Slack' })).report).toMatchObject({ renameProblems: [{ id: 'webhook', problem: 'taken' }], divergent: true });
    expect(apply((graph) => replaceNode(graph, 'webhook', { name: '' })).report.renameProblems).toEqual([{ id: 'webhook', problem: 'empty' }]);
  });

  it('disables and enables, but never enables a placeholder', () => {
    const disabled = apply((graph) => replaceNode(graph, 'slack', { disabled: true }));
    expect(findNode(disabled.document, 'slack')?.disabled).toBe(true);

    const document = exampleFlowDocument();
    document.nodes.push({ id: 'old', name: 'Old node', element: 'task', type: 'orcon.unsupported@1', parameters: {} });
    const before = toCanvasGraph(document);
    const after = change(before, (graph) => replaceNode(graph, 'old', { disabled: false }));
    expect(applyCanvasEdit(document, before, after).divergent).toBe(true);
    expect(findNode(document, 'old')).not.toHaveProperty('disabled');
  });

  it('does not store a pin and asks for a rebuild', () => {
    const { report, document } = apply((graph) => replaceNode(graph, 'slack', { pinned: true }));
    expect(report).toMatchObject({ pinIgnored: ['slack'], divergent: true });
    expect(JSON.stringify(document)).not.toContain('pinned');
  });

  it('removes nodes with their connections', () => {
    const { document } = apply((graph) => {
      graph.nodes = graph.nodes.filter((node) => node.id !== 'inventory');
      graph.edges = graph.edges.filter((edge) => edge.source !== 'inventory' && edge.target !== 'inventory');
    });
    expect(document.nodes.map((node) => node.id)).not.toContain('inventory');
    expect(document.edges.map((edge) => edge.id)).toEqual(['e1', 'e3', 'e5']);
  });

  it('connects and disconnects', () => {
    const connected = apply((graph) => {
      graph.edges.push({ id: 'edge-inventory-rejected-1', source: 'inventory', sourceOutput: 'main', target: 'rejected', targetInput: 'main' });
    });
    expect(connected.document.edges.at(-1)).toEqual({ id: 'edge-inventory-rejected-1', from: { node: 'inventory', port: 'main' }, to: { node: 'rejected' }, kind: 'SEQUENCE' });
    expect(connected.report.divergent).toBe(false);

    const removed = apply((graph) => {
      graph.edges = graph.edges.filter((edge) => edge.id !== 'e3');
    });
    expect(removed.document.edges.map((edge) => edge.id)).toEqual(['e1', 'e2', 'e4', 'e5']);
  });

  it('refuses a connection that closes a cycle or goes into a trigger, and asks for a rebuild', () => {
    const cycle = apply((graph) => {
      graph.edges.push({ id: 'c1', source: 'slack', sourceOutput: 'main', target: 'validate', targetInput: 'main' });
    });
    expect(cycle.report).toMatchObject({ edgeProblems: [{ id: 'c1', problem: 'cycle' }], divergent: true });
    expect(cycle.document.edges).toHaveLength(5);

    const into = apply((graph) => {
      graph.edges.push({ id: 'c2', source: 'rejected', sourceOutput: 'main', target: 'webhook', targetInput: 'main' });
    });
    expect(into.report.edgeProblems).toEqual([{ id: 'c2', problem: 'into-start' }]);
  });

  it('gives a connection with an id the format does not allow a new one and asks for a rebuild', () => {
    const { document, report } = apply((graph) => {
      graph.edges.push({ id: 'a connection!', source: 'inventory', sourceOutput: 'main', target: 'rejected', targetInput: 'main' });
    });
    expect(document.edges.at(-1)?.id).toBe('edge-1');
    expect(report.divergent).toBe(true);
  });

  it('records a back edge into a loop node as BACK', () => {
    const document = exampleFlowDocument();
    document.nodes.push({ id: 'again', name: 'Again', element: 'task', type: 'orcon.loop@1', parameters: {} });
    document.edges.push({ id: 'l1', from: { node: 'validate', port: 'true' }, to: { node: 'again' }, kind: 'SEQUENCE' }, { id: 'l2', from: { node: 'again', port: 'loop' }, to: { node: 'inventory' }, kind: 'SEQUENCE' });
    const before = toCanvasGraph(document);
    const after = change(before, (graph) => {
      graph.edges.push({ id: 'l3', source: 'inventory', sourceOutput: 'main', target: 'again', targetInput: 'main' });
    });
    applyCanvasEdit(document, before, after);
    expect(document.edges.at(-1)).toMatchObject({ id: 'l3', kind: 'BACK' });
  });

  it('adds, changes, moves, resizes and removes sticky notes', () => {
    const added = apply((graph) => {
      graph.notes?.push({ id: 'note-2', position: { x: 100, y: 500 }, size: { w: 220, h: 120 }, color: 1, body: 'x'.repeat(12_000) });
    });
    expect(added.document.editor.stickyNotes?.[1]?.body).toHaveLength(10_000);
    expect(added.document.editor.stickyNotes?.[1]).not.toHaveProperty('title');

    const changed = apply((graph) => {
      const note = graph.notes?.[0];
      if (!note || !graph.notes) throw new Error('no note');
      graph.notes[0] = { ...note, position: { x: 60, y: 360 }, size: { w: 300, h: 140 }, color: 5, body: 'new', title: undefined };
    });
    expect(changed.document.editor.stickyNotes?.[0]).toEqual({ id: 'note-1', x: 60, y: 360, w: 300, h: 140, color: 5, body: 'new' });

    const titled = apply((graph) => {
      const note = graph.notes?.[0];
      if (!note || !graph.notes) throw new Error('no note');
      graph.notes[0] = { ...note, title: 'Intake' };
    });
    expect(titled.document.editor.stickyNotes?.[0]?.title).toBe('Intake');

    const removed = apply((graph) => {
      graph.notes = [];
    });
    expect(removed.document.editor.stickyNotes).toEqual([]);
  });

  it('writes the position of every node, so the next edit does not move the ones that never had one', () => {
    const document = exampleFlowDocument();
    document.editor = {};
    const before = toCanvasGraph(document);
    const after = change(before, (graph) => replaceNode(graph, 'done', { position: { x: 900, y: 60 } }));
    applyCanvasEdit(document, before, after);
    expect(document.editor.nodes?.['inventory']).toEqual({ x: 400, y: 0 });
    expect(document.editor.nodes?.['done']).toEqual({ x: 900, y: 60 });
  });

  it('reports no change for the same graph', () => {
    const document = exampleFlowDocument();
    const graph = toCanvasGraph(document);
    expect(applyCanvasEdit(document, graph, graph)).toEqual({ renamed: [], renameProblems: [], edgeProblems: [], adjustedNames: [], pinIgnored: [], divergent: false });
  });
});
