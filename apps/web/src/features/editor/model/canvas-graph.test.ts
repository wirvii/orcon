// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { clientToFlow, fallbackPositions, graphAgrees, parseViewportTransform, snap, toCanvasGraph } from './canvas-graph';
import { exampleFlowDocument } from './example-flow';
import { cloneDocument } from './flow-document';

describe('the canvas view of a document', () => {
  it('draws the sample flow: a trigger, an If with two outputs, tiles and an end', () => {
    const graph = toCanvasGraph(exampleFlowDocument());
    const byId = new Map(graph.nodes.map((node) => [node.id, node]));
    expect(graph.nodes).toHaveLength(6);
    expect(byId.get('webhook')).toMatchObject({ kind: 'trigger', icon: 'webhook', subtitle: 'POST /orders', inputs: [], outputs: [{ id: 'main' }], position: { x: 40, y: 160 } });
    expect(byId.get('validate')).toMatchObject({ kind: 'logic', logicType: 'if', subtitle: '$json.total > 0', outputs: [{ id: 'true', label: 'true' }, { id: 'false', label: 'false' }] });
    expect(byId.get('inventory')).toMatchObject({ kind: 'action', icon: 'globe', subtitle: 'GET inventory.example.test' });
    expect(byId.get('slack')).toMatchObject({ subtitle: 'POST hooks.slack.example.test' });
    expect(byId.get('done')).toMatchObject({ outputs: [] });
    expect(graph.edges.map((edge) => [edge.id, edge.source, edge.sourceOutput, edge.target, edge.targetInput])).toContainEqual(['e2', 'validate', 'true', 'inventory', 'main']);
    expect(graph.notes).toEqual([{ id: 'note-1', position: { x: 40, y: 340 }, size: { w: 260, h: 120 }, color: 2, title: 'Order intake', body: expect.any(String) as string }]);
  });

  it('adds the ports the connections use and the error output of a node that has one', () => {
    const document = exampleFlowDocument();
    const slack = document.nodes.find((node) => node.id === 'slack');
    if (!slack) throw new Error('missing');
    slack['onError'] = 'ERROR_OUTPUT';
    document.edges.push({ id: 'e9', from: { node: 'inventory', port: 'case:vip' }, to: { node: 'done' }, kind: 'SEQUENCE' });
    const graph = toCanvasGraph(document);
    expect(graph.nodes.find((node) => node.id === 'slack')?.outputs.map((port) => port.id)).toEqual(['main', 'error']);
    expect(graph.nodes.find((node) => node.id === 'inventory')?.outputs.map((port) => port.id)).toEqual(['main', 'case:vip']);
  });

  it('shows the inert placeholder as a disabled tile and a loop-back as such', () => {
    const document = exampleFlowDocument();
    document.nodes.push({ id: 'old', name: 'Old node', element: 'task', type: 'orcon.unsupported@1', parameters: {} });
    document.nodes.push({ id: 'again', name: 'Again', element: 'task', type: 'orcon.loop@1', parameters: {} });
    document.edges.push({ id: 'b1', from: { node: 'slack', port: 'main' }, to: { node: 'again' }, kind: 'BACK' });
    const graph = toCanvasGraph(document);
    expect(graph.nodes.find((node) => node.id === 'old')).toMatchObject({ kind: 'placeholder', disabled: true, icon: 'help' });
    expect(graph.edges.find((edge) => edge.id === 'b1')?.loopBack).toBe(true);
    expect(graph.nodes.find((node) => node.id === 'again')).toMatchObject({ kind: 'logic', logicType: 'loop', outputs: [{ id: 'loop' }, { id: 'done' }] });
  });

  it('uses the description of the type when the parameters give no line', () => {
    const document = exampleFlowDocument();
    document.nodes.push({ id: 'plain', name: 'Plain', element: 'task', type: 'orcon.set@1', parameters: {} });
    const graph = toCanvasGraph(document, (node) => `about ${node.type}`);
    expect(graph.nodes.find((node) => node.id === 'plain')?.subtitle).toBe('about orcon.set@1');
    expect(graph.nodes.find((node) => node.id === 'webhook')?.subtitle).toBe('POST /orders');
  });

  it('places nodes that have no recorded position in layers, one column per step', () => {
    const document = exampleFlowDocument();
    document.editor = {};
    const positions = fallbackPositions(document);
    expect(positions.get('webhook')).toEqual({ x: 0, y: 0 });
    expect(positions.get('validate')).toEqual({ x: 200, y: 0 });
    expect(positions.get('inventory')).toEqual({ x: 400, y: 0 });
    expect(positions.get('rejected')).toEqual({ x: 400, y: 160 });
    expect(positions.get('done')).toEqual({ x: 800, y: 0 });
    const graph = toCanvasGraph(document);
    expect(graph.nodes.find((node) => node.id === 'slack')?.position).toEqual({ x: 600, y: 0 });
  });

  it('survives a cycle in a document it is asked to draw', () => {
    const document = exampleFlowDocument();
    document.editor = {};
    document.edges.push({ id: 'cycle', from: { node: 'done', port: 'main' }, to: { node: 'webhook' }, kind: 'SEQUENCE' });
    expect(() => toCanvasGraph(document)).not.toThrow();
  });
});

describe('does the canvas show the document', () => {
  it('agrees with its own drawing', () => {
    const document = exampleFlowDocument();
    expect(graphAgrees(document, toCanvasGraph(document))).toBe(true);
  });

  it('disagrees as soon as a name, a position, a connection, a note or a pin differs', () => {
    const document = exampleFlowDocument();
    const mutations: ((graph: ReturnType<typeof toCanvasGraph>) => void)[] = [
      (graph) => void graph.nodes.pop(),
      (graph) => void graph.edges.pop(),
      (graph) => {
        const node = graph.nodes[0];
        if (node) node.name = 'Other';
      },
      (graph) => {
        const node = graph.nodes[0];
        if (node) node.position = { x: 1, y: 1 };
      },
      (graph) => {
        const node = graph.nodes[1];
        if (node) node.disabled = true;
      },
      (graph) => {
        const node = graph.nodes[1];
        if (node) node.pinned = true;
      },
      (graph) => {
        const edge = graph.edges[0];
        if (edge) edge.target = 'done';
      },
      (graph) => {
        const note = graph.notes?.[0];
        if (note) note.body = 'changed';
      },
      (graph) => void graph.notes?.pop(),
    ];
    for (const mutate of mutations) {
      const graph = toCanvasGraph(cloneDocument(document));
      mutate(graph);
      expect(graphAgrees(document, graph)).toBe(false);
    }
  });
});

describe('geometry', () => {
  it('snaps to the 20 px grid', () => {
    expect(snap(29)).toBe(20);
    expect(snap(31)).toBe(40);
    expect(snap(-9)).toBe(-0);
  });

  it('reads the viewport the canvas draws with and converts a point of the page into canvas coordinates', () => {
    expect(parseViewportTransform('translate(120px, -40.5px) scale(0.8)')).toEqual({ x: 120, y: -40.5, zoom: 0.8 });
    expect(parseViewportTransform('translate(0px,0px) scale(1)')).toEqual({ x: 0, y: 0, zoom: 1 });
    expect(parseViewportTransform('none')).toBeUndefined();
    expect(parseViewportTransform('translate(1px, 2px) scale(0)')).toBeUndefined();
    expect(clientToFlow({ x: 220, y: 160 }, { left: 100, top: 50 }, { x: 20, y: 10, zoom: 0.5 })).toEqual({ x: 200, y: 200 });
  });
});
