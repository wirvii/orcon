// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import reference from '../../../../perf/fixtures/reference-flow-500.json';
import { PERF_FLOW_ID } from '../model/example-flow';
import { flowDocumentSchema, type FlowDocument, type FlowEdge, type FlowNode } from '../model/flow-document';
import { START_TYPE } from '../model/node-types';

function toNode(source: (typeof reference.nodes)[number]): FlowNode {
  const base = { id: source.id, name: source.name };
  if (source.kind === 'trigger') return { ...base, element: 'startEvent', type: START_TYPE, parameters: { method: 'POST', path: '/start' } };
  if ('logic' in source && source.logic === 'merge') return { ...base, element: 'task', type: 'orcon.merge@1', parameters: {}, join: { mode: 'ALL', data: 'APPEND' } };
  if (source.kind === 'control') return { ...base, element: 'exclusiveGateway', type: 'orcon.if@1', parameters: { condition: '' }, routing: 'EXCLUSIVE' };
  return { ...base, element: 'serviceTask', type: 'orcon.http@1', parameters: { method: 'GET', url: '' } };
}

/**
 * The seeded reference flow of the performance measurement (orcon-web PF-4: 500 nodes, 560
 * connections, 12 of them loop-backs) as a flow document, so the measurement opens it through the
 * real editor route. Only the performance build can reach it; the production build leaves it out.
 */
export function referenceFlowDocument(): FlowDocument {
  const edges = reference.edges.map(
    (edge): FlowEdge => ({
      id: edge.id,
      from: { node: edge.source, port: edge.sourceOutput },
      to: { node: edge.target },
      kind: 'loopBack' in edge ? 'BACK' : 'SEQUENCE',
    }),
  );
  return flowDocumentSchema.parse({
    format: 'orcon/flow@1',
    id: PERF_FLOW_ID,
    name: 'Reference flow with 500 nodes',
    settings: {},
    variables: [],
    nodes: reference.nodes.map(toNode),
    edges,
    editor: { nodes: Object.fromEntries(reference.nodes.map((node) => [node.id, node.position])) },
  });
}
