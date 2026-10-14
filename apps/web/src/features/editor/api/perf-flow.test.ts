// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { PERF_FLOW_ID } from '../model/example-flow';
import { graphAgrees, toCanvasGraph } from '../model/canvas-graph';

import { referenceFlowDocument } from './perf-flow';

describe('the 500-node reference flow as a flow document', () => {
  const document = referenceFlowDocument();

  it('has the size the performance budget names: 500 nodes, 560 connections of which 12 loop back', () => {
    expect(document.id).toBe(PERF_FLOW_ID);
    expect(document.nodes).toHaveLength(500);
    expect(document.edges).toHaveLength(560);
    expect(document.edges.filter((edge) => edge.kind === 'BACK')).toHaveLength(12);
  });

  it('draws every node at the position the reference flow gives it, and the canvas agrees with it', () => {
    const graph = toCanvasGraph(document);
    expect(graph.nodes).toHaveLength(500);
    expect(graph.nodes[0]).toMatchObject({ id: 'n001', kind: 'trigger', position: { x: 0, y: 0 } });
    expect(graph.nodes.filter((node) => node.kind === 'logic')).toHaveLength(80);
    expect(graphAgrees(document, graph)).toBe(true);
  });
});
