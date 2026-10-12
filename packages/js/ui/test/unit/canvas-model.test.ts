// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { exampleCatalog, exampleDraft } from '../../src/canvas/fixtures';
import {
  GRID,
  addNode,
  connect,
  connectError,
  itemCounts,
  moveNodes,
  nextFocus,
  pushHistory,
  readingOrder,
  redoHistory,
  removeNodes,
  snap,
  startHistory,
  undoHistory,
  type Graph,
} from '../../src/canvas/model';
import { exampleGraph } from '../../src/canvas/fixtures';
import { loopBackPath } from '../../src/canvas/Edge';

describe('canvas model', () => {
  it('snaps positions to the 8 px grid', () => {
    expect(snap(13)).toBe(16);
    expect(snap(11)).toBe(8);
    expect(GRID).toBe(8);
  });

  it('moves focus to the nearest node in the arrow direction', () => {
    expect(nextFocus(exampleDraft, 'order-received', 'right')).toBe('check-stock');
    expect(nextFocus(exampleDraft, 'check-stock', 'left')).toBe('order-received');
    expect(nextFocus(exampleDraft, 'check-stock', 'right')).toBe('in-stock');
    // From the branch, up goes to the node above (charge) and down to the one below (back order).
    expect(nextFocus(exampleDraft, 'in-stock', 'right')).toBeDefined();
    expect(nextFocus(exampleDraft, 'order-received', 'left')).toBeUndefined();
  });

  it('lists nodes in reading order', () => {
    expect(readingOrder(exampleDraft)[0]?.id).toBe('order-received');
  });

  it('moves nodes by grid steps and keeps them on the grid', () => {
    const moved = moveNodes(exampleDraft, ['check-stock'], GRID, -GRID * 4);
    const before = exampleDraft.nodes.find((node) => node.id === 'check-stock')!.position;
    const after = moved.nodes.find((node) => node.id === 'check-stock')!.position;
    expect(after).toEqual({ x: before.x + GRID, y: before.y - GRID * 4 });
    expect(after.x % GRID).toBe(0);
    // The original graph is not changed.
    expect(exampleDraft.nodes.find((node) => node.id === 'check-stock')!.position).toEqual(before);
  });

  it('adds a node from the catalog with a unique id and no input on a trigger', () => {
    const trigger = exampleCatalog.find((item) => item.kind === 'trigger')!;
    const first = addNode(exampleDraft, trigger, { x: 13, y: 21 });
    const second = addNode(first.graph, trigger, { x: 0, y: 0 });
    expect(first.id).not.toBe(second.id);
    const node = first.graph.nodes.find((candidate) => candidate.id === first.id)!;
    expect(node.inputs).toEqual([]);
    expect(node.position).toEqual({ x: 16, y: 24 });
  });

  it('removes nodes together with their connections', () => {
    const result = removeNodes(exampleDraft, ['in-stock']);
    expect(result.nodes.some((node) => node.id === 'in-stock')).toBe(false);
    expect(result.edges.every((edge) => edge.source !== 'in-stock' && edge.target !== 'in-stock')).toBe(true);
    expect(result.edges.length).toBeLessThan(exampleDraft.edges.length);
  });

  describe('connections', () => {
    const request = { source: 'order-received', sourceOutput: 'main', target: 'send-receipt', targetInput: 'main' };

    it('connects an output to an input', () => {
      const result = connect(exampleDraft, request);
      expect(result.error).toBeUndefined();
      expect(result.graph.edges).toHaveLength(exampleDraft.edges.length + 1);
    });

    it.each([
      ['a node to itself', { ...request, target: 'order-received' }, 'same-node'],
      ['into a trigger', { source: 'check-stock', sourceOutput: 'main', target: 'order-received', targetInput: 'main' }, 'trigger-input'],
      ['an unknown node', { ...request, target: 'nope' }, 'unknown-node'],
      ['an unknown port', { ...request, sourceOutput: 'nope' }, 'unknown-port'],
      ['a connection that exists', { source: 'order-received', sourceOutput: 'main', target: 'check-stock', targetInput: 'main' }, 'duplicate'],
    ] as const)('rejects %s', (_label, bad, error) => {
      expect(connectError(exampleDraft, bad)).toBe(error);
      expect(connect(exampleDraft, bad).graph).toBe(exampleDraft);
    });

    it('marks a connection to an earlier node as a loop back', () => {
      const result = connect(exampleDraft, { source: 'send-receipt', sourceOutput: 'main', target: 'check-stock', targetInput: 'main' });
      expect(result.graph.edges.at(-1)?.loopBack).toBe(true);
    });
  });

  it('adds up the items that arrived at a node and left it', () => {
    expect(itemCounts(exampleGraph, 'check-stock')).toEqual({ received: 12 + 3, sent: 12 });
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

  it('routes a loop back under the flow', () => {
    const { path, labelY } = loopBackPath(800, 100, 200, 100);
    expect(path.startsWith('M 800,100')).toBe(true);
    expect(labelY).toBeGreaterThan(100);
  });
});
