// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { edgeGeometry } from '../../src/canvas/canvas-edge/CanvasEdge';
import { exampleCatalog, exampleDraft, exampleGraph } from '../../src/canvas/fixtures';
import {
  COLUMN_PITCH,
  GRID,
  addNode,
  addNodeAfter,
  addNote,
  connect,
  connectError,
  danglingOutputs,
  downstream,
  freeInput,
  insertOnEdge,
  itemCounts,
  moveNodes,
  nextFocus,
  nodeSize,
  portOffsets,
  pushHistory,
  readingOrder,
  redoHistory,
  removeNodes,
  snap,
  startHistory,
  undoHistory,
  type Graph,
} from '../../src/canvas/model';

const item = (id: string) => exampleCatalog.find((candidate) => candidate.id === id)!;

describe('canvas model', () => {
  it('snaps positions to the 20 px grid', () => {
    expect(GRID).toBe(20);
    expect(snap(29)).toBe(20);
    expect(snap(31)).toBe(40);
    expect(COLUMN_PITCH).toBe(200);
  });

  it('sizes a tile 100 x 100, grows it beyond two outputs and makes the agent tile 200 wide', () => {
    const two = exampleDraft.nodes.find((node) => node.id === 'validate')!;
    expect(nodeSize(two)).toEqual({ w: 100, h: 100 });
    const many = { kind: 'logic' as const, inputs: [{ id: 'main' }], outputs: Array.from({ length: 5 }, (_, index) => ({ id: String(index) })) };
    // max(100, 26 x outputs + 22)
    expect(nodeSize(many)).toEqual({ w: 100, h: 152 });
    expect(nodeSize({ kind: 'agent', inputs: [], outputs: [] })).toEqual({ w: 200, h: 100 });
  });

  it('places ports: one centered, two at 24 and 76, more at a 26 px pitch', () => {
    expect(portOffsets(1, 100)).toEqual([50]);
    expect(portOffsets(2, 100)).toEqual([24, 76]);
    expect(portOffsets(3, 100)).toEqual([24, 50, 76]);
    expect(portOffsets(4, 126)).toEqual([24, 50, 76, 102]);
  });

  it('moves focus to the nearest node in the arrow direction, sticky notes included', () => {
    expect(nextFocus(exampleDraft, 'webhook', 'right')).toBe('validate');
    expect(nextFocus(exampleDraft, 'validate', 'left')).toBe('webhook');
    expect(nextFocus(exampleDraft, 'webhook', 'left')).toBeUndefined();
    // The note sits below the trigger.
    expect(nextFocus(exampleDraft, 'webhook', 'down')).toBe('note-1');
  });

  it('lists nodes in reading order', () => {
    expect(readingOrder(exampleDraft)[0]?.id).toBe('webhook');
  });

  it('moves nodes by grid steps and keeps them on the grid', () => {
    const moved = moveNodes(exampleDraft, ['validate'], GRID, -GRID * 4);
    const before = exampleDraft.nodes.find((node) => node.id === 'validate')!.position;
    const after = moved.nodes.find((node) => node.id === 'validate')!.position;
    expect(after).toEqual({ x: before.x + GRID, y: before.y - GRID * 4 });
    expect(after.x % GRID).toBe(0);
    // The original graph is not changed.
    expect(exampleDraft.nodes.find((node) => node.id === 'validate')!.position).toEqual(before);
  });

  it('adds a node from the catalog with a unique id and no input on a trigger', () => {
    const trigger = exampleCatalog.find((candidate) => candidate.kind === 'trigger')!;
    const first = addNode(exampleDraft, trigger, { x: 13, y: 31 });
    const second = addNode(first.graph, trigger, { x: 0, y: 0 });
    expect(first.id).not.toBe(second.id);
    const node = first.graph.nodes.find((candidate) => candidate.id === first.id)!;
    expect(node.inputs).toEqual([]);
    expect(node.position).toEqual({ x: 20, y: 40 });
  });

  it('adds a node after an output: one column to the right, same row, connected', () => {
    const result = addNodeAfter(exampleDraft, item('send-email'), 'http', 'main');
    const source = exampleDraft.nodes.find((node) => node.id === 'http')!;
    const added = result.graph.nodes.find((node) => node.id === result.id)!;
    // The slot to the right of HTTP Request is taken (Slack), so the node moves down to the first free spot.
    expect(added.position.x).toBe(source.position.x + COLUMN_PITCH);
    expect(added.position.x % GRID).toBe(0);
    expect(result.graph.edges.some((edge) => edge.source === 'http' && edge.target === result.id)).toBe(true);
  });

  it('lands on the same row when the slot is free', () => {
    const result = addNodeAfter(exampleDraft, item('send-email'), 'merge', 'main');
    const source = exampleDraft.nodes.find((node) => node.id === 'merge')!;
    expect(result.graph.nodes.find((node) => node.id === result.id)!.position).toEqual({ x: source.position.x + COLUMN_PITCH, y: source.position.y });
  });

  it('inserts a node on a connection: splits it, reconnects it and moves what follows one column right', () => {
    const result = insertOnEdge(exampleDraft, 'e1', item('http-request'));
    const edges = result.graph.edges;
    expect(edges.some((edge) => edge.id === 'e1')).toBe(false);
    expect(edges.some((edge) => edge.source === 'webhook' && edge.target === result.id)).toBe(true);
    expect(edges.some((edge) => edge.source === result.id && edge.target === 'validate')).toBe(true);
    const before = exampleDraft.nodes.find((node) => node.id === 'validate')!.position;
    const after = result.graph.nodes.find((node) => node.id === 'validate')!.position;
    expect(after.x).toBe(before.x + COLUMN_PITCH);
    // The new node takes the target's old column.
    expect(result.graph.nodes.find((node) => node.id === result.id)!.position.x).toBe(before.x);
  });

  it('finds what follows a node', () => {
    expect([...downstream(exampleDraft, 'validate')].sort()).toEqual(['gmail', 'http', 'merge', 'slack', 'validate']);
  });

  it('removes nodes together with their connections', () => {
    const result = removeNodes(exampleDraft, ['validate']);
    expect(result.nodes.some((node) => node.id === 'validate')).toBe(false);
    expect(result.edges.every((edge) => edge.source !== 'validate' && edge.target !== 'validate')).toBe(true);
    expect(result.edges.length).toBeLessThan(exampleDraft.edges.length);
  });

  it('adds a sticky note on the grid with a unique id', () => {
    const first = addNote(exampleDraft, { x: 33, y: 47 }, 'Hello');
    expect(first.graph.notes?.at(-1)).toMatchObject({ position: { x: 40, y: 40 }, color: 1, body: 'Hello' });
    expect(addNote(first.graph, { x: 0, y: 0 }, '').id).not.toBe(first.id);
  });

  it('knows which outputs and inputs are free', () => {
    expect(danglingOutputs(exampleDraft, 'merge')).toEqual(['main']);
    expect(danglingOutputs(exampleDraft, 'webhook')).toEqual([]);
    expect(freeInput(exampleDraft, 'merge')).toBe('a');
  });

  describe('connections', () => {
    const request = { source: 'webhook', sourceOutput: 'main', target: 'merge', targetInput: 'a' };

    it('connects an output to an input', () => {
      const result = connect(exampleDraft, request);
      expect(result.error).toBeUndefined();
      expect(result.graph.edges).toHaveLength(exampleDraft.edges.length + 1);
    });

    it.each([
      ['a node to itself', { ...request, target: 'webhook' }, 'same-node'],
      ['into a trigger', { source: 'http', sourceOutput: 'main', target: 'webhook', targetInput: 'main' }, 'trigger-input'],
      ['an unknown node', { ...request, target: 'nope' }, 'unknown-node'],
      ['an unknown port', { ...request, sourceOutput: 'nope' }, 'unknown-port'],
      ['a connection that exists', { source: 'webhook', sourceOutput: 'main', target: 'validate', targetInput: 'main' }, 'duplicate'],
    ] as const)('rejects %s', (_label, bad, error) => {
      expect(connectError(exampleDraft, bad)).toBe(error);
      expect(connect(exampleDraft, bad).graph).toBe(exampleDraft);
    });

    it('marks a connection to an earlier node as a loop back', () => {
      const result = connect(exampleDraft, { source: 'merge', sourceOutput: 'main', target: 'validate', targetInput: 'main' });
      expect(result.graph.edges.at(-1)?.loopBack).toBe(true);
    });
  });

  it('adds up the items that arrived at a node and left it', () => {
    expect(itemCounts(exampleGraph, 'http')).toEqual({ received: 1, sent: 3 });
  });

  describe('history', () => {
    const a: Graph = { nodes: [], edges: [] };
    const b: Graph = { ...a, nodes: exampleDraft.nodes.slice(0, 1) };
    const c: Graph = { ...a, nodes: exampleDraft.nodes.slice(0, 2) };

    it('undoes and redoes', () => {
      let history = startHistory(a);
      history = pushHistory(history, b);
      history = pushHistory(history, c);
      history = undoHistory(history);
      expect(history.present).toBe(b);
      history = undoHistory(history);
      expect(history.present).toBe(a);
      expect(undoHistory(history)).toBe(history);
      history = redoHistory(history);
      expect(history.present).toBe(b);
    });

    it('drops the redo stack when a new change is made', () => {
      let history = pushHistory(pushHistory(startHistory(a), b), c);
      history = undoHistory(history);
      history = pushHistory(history, a);
      expect(history.future).toEqual([]);
    });

    it('ignores a change that is the same graph and keeps a bounded past', () => {
      const history = startHistory(a);
      expect(pushHistory(history, a)).toBe(history);
      let long = startHistory(a);
      for (let index = 0; index < 10; index += 1) long = pushHistory(long, { ...a }, 3);
      expect(long.past).toHaveLength(3);
    });
  });

  describe('edge geometry', () => {
    it('leaves the source port with a gap, ends at the target port and bends with a horizontal tangent', () => {
      const { d, tip } = edgeGeometry(100, 50, 300, 150);
      expect(d.startsWith('M 106,50')).toBe(true);
      expect(tip).toEqual({ x: 292, y: 150 });
      // Control offset max(30, dx / 2): dx = 186 -> 93.
      expect(d).toContain('C 199,50 199,150 292,150');
    });

    it('keeps a minimum control offset of 30 when the nodes are close', () => {
      const { d } = edgeGeometry(100, 50, 140, 150);
      expect(d).toContain('C 136,50 102,150 132,150');
    });

    it('routes a loop back under the flow', () => {
      const { d, mid } = edgeGeometry(800, 100, 200, 100, true);
      expect(d.startsWith('M 806,100')).toBe(true);
      expect(mid.y).toBeGreaterThan(100);
    });
  });
});
