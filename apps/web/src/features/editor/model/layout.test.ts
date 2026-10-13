// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';

import { toCanvasGraph } from './canvas-graph';
import { exampleFlowDocument } from './example-flow';
import { layoutGraph, type LayoutInput } from './layout';
import { computeLayout } from './layout-client';

/** The layout input for a document: tiles of 100, forward connections only. */
function inputOf(only?: ReadonlySet<string>): LayoutInput {
  const document = exampleFlowDocument();
  const graph = toCanvasGraph(document);
  const keep = (id: string) => only === undefined || only.has(id);
  return {
    nodes: graph.nodes.filter((node) => keep(node.id)).map((node) => ({ id: node.id, w: 100, h: 100, position: node.position })),
    edges: graph.edges
      .filter((edge) => !edge.loopBack)
      .map((edge) => ({ from: edge.source, to: edge.target, port: Math.max(0, graph.nodes.find((node) => node.id === edge.source)?.outputs.findIndex((port) => port.id === edge.sourceOutput) ?? 0) })),
    notes: (graph.notes ?? []).filter((note) => keep(note.id)).map((note) => ({ id: note.id, w: note.size.w, h: note.size.h, position: note.position })),
  };
}

const positions = (output: ReturnType<typeof layoutGraph>) => new Map(output);

describe('tidying up', () => {
  it('lays the flow out in columns from left to right, one per step, on the 20 px grid', () => {
    const placed = positions(layoutGraph(inputOf()));
    const column = (id: string) => placed.get(id)?.x ?? Number.NaN;
    expect(column('webhook')).toBeLessThan(column('validate'));
    expect(column('validate')).toBeLessThan(column('inventory'));
    expect(column('inventory')).toBeLessThan(column('slack'));
    expect(column('slack')).toBeLessThan(column('done'));
    expect(column('rejected')).toBe(column('inventory'));
    // A column is a tile and a gap of one tile.
    expect(column('validate') - column('webhook')).toBe(200);
    for (const point of placed.values()) {
      expect(point.x % 20).toBe(0);
      expect(point.y % 20).toBe(0);
    }
  });

  it('puts the target of the first output above the target of the second, so the connections of an If do not cross', () => {
    const placed = positions(layoutGraph(inputOf()));
    expect(placed.get('inventory')?.y).toBeLessThan(placed.get('rejected')?.y ?? Number.NaN);
    // The same holds when the second output's target comes first in the document.
    const input = inputOf();
    input.edges.reverse();
    input.nodes.reverse();
    const reversed = positions(layoutGraph(input));
    expect(reversed.get('inventory')?.y).toBeLessThan(reversed.get('rejected')?.y ?? Number.NaN);
  });

  it('keeps the flow where it is: the top left corner stays', () => {
    const placed = positions(layoutGraph(inputOf()));
    const nodes = inputOf().nodes;
    const left = Math.min(...nodes.map((node) => node.position.x));
    const top = Math.min(...nodes.map((node) => node.position.y));
    expect(Math.min(...nodes.map((node) => placed.get(node.id)?.x ?? Infinity))).toBe(left);
    expect(Math.min(...nodes.map((node) => placed.get(node.id)?.y ?? Infinity))).toBe(top);
  });

  it('keeps a sticky note at the same distance from the node it was nearest to', () => {
    const input = inputOf();
    const placed = positions(layoutGraph(input));
    // The note is nearest to the webhook (40,160) and sits at (40,340): 180 below it.
    const webhook = placed.get('webhook');
    expect(placed.get('note-1')).toEqual({ x: webhook?.x, y: (webhook?.y ?? 0) + 180 });
  });

  it('lays out only the selection and leaves the others alone', () => {
    const selection = new Set(['inventory', 'slack', 'done']);
    const output = layoutGraph(inputOf(selection));
    expect([...positions(output).keys()].sort()).toEqual(['done', 'inventory', 'slack']);
  });

  it('does nothing for an empty flow and ignores connections to nodes that are not laid out', () => {
    expect(layoutGraph({ nodes: [], edges: [{ from: 'a', to: 'b' }], notes: [] })).toEqual([]);
    const one = layoutGraph({ nodes: [{ id: 'a', w: 100, h: 100, position: { x: 33, y: 47 } }], edges: [{ from: 'a', to: 'ghost' }, { from: 'a', to: 'a' }], notes: [] });
    expect(one).toEqual([['a', { x: 40, y: 40 }]]);
  });

  it('lays out a large flow quickly enough to be worth a worker', () => {
    const nodes = Array.from({ length: 500 }, (_, index) => ({ id: `n${String(index)}`, w: 100, h: 100, position: { x: index * 7, y: index * 3 } }));
    const edges = nodes.slice(1).map((node, index) => ({ from: `n${String(Math.floor(index / 2))}`, to: node.id }));
    const started = performance.now();
    const output = layoutGraph({ nodes, edges, notes: [] });
    const elapsed = performance.now() - started;
    expect(output).toHaveLength(500);
    // The budget is 300 ms in the worker (R-ED-PERF-11); this only guards against a layout that is an order of magnitude slower.
    expect(elapsed).toBeLessThan(3_000);
  });
});

describe('running the layout', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('computes in this thread where there is no worker', async () => {
    const result = await computeLayout(inputOf());
    expect(result.get('validate')).toEqual(positions(layoutGraph(inputOf())).get('validate'));
  });

  it('asks a worker and uses its answer', async () => {
    const answer = [['a', { x: 20, y: 40 }]];
    const terminated = vi.fn();
    class FakeWorker {
      onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
      onerror: (() => void) | null = null;
      postMessage() {
        queueMicrotask(() => this.onmessage?.(new MessageEvent('message', { data: answer })));
      }
      terminate = terminated;
    }
    vi.stubGlobal('Worker', FakeWorker);
    const result = await computeLayout(inputOf());
    expect([...result]).toEqual(answer);
    expect(terminated).toHaveBeenCalled();
  });

  it('computes in this thread when the worker answers nonsense, fails, or cannot start', async () => {
    const expected = positions(layoutGraph(inputOf())).get('validate');
    class Nonsense {
      onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
      postMessage() {
        queueMicrotask(() => this.onmessage?.(new MessageEvent('message', { data: 'nonsense' })));
      }
      terminate() {
        return undefined;
      }
    }
    vi.stubGlobal('Worker', Nonsense);
    expect((await computeLayout(inputOf())).get('validate')).toEqual(expected);

    class Failing {
      onerror: (() => void) | null = null;
      postMessage() {
        queueMicrotask(() => this.onerror?.());
      }
      terminate() {
        return undefined;
      }
    }
    vi.stubGlobal('Worker', Failing);
    expect((await computeLayout(inputOf())).get('validate')).toEqual(expected);

    vi.stubGlobal('Worker', function Broken() {
      throw new Error('blocked');
    });
    expect((await computeLayout(inputOf())).get('validate')).toEqual(expected);
  });
});
