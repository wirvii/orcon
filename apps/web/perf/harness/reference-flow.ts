// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Graph, GraphEdge, GraphNode } from '@orcon/ui';

import reference from '../fixtures/reference-flow-500.json';

/** The icons the reference flow uses; the generator picks from this list. */
type ReferenceIcon = NonNullable<GraphNode['icon']>;

/** The design system's flow model for the committed reference flow (500 nodes, 560 connections). */
export function referenceGraph(): Graph {
  const nodes: GraphNode[] = reference.nodes.map((node) => {
    const base = {
      id: node.id,
      name: node.name,
      subtitle: node.subtitle,
      inputs: node.inputs.map((id) => ({ id })),
      outputs: node.outputs.map((id) => ({ id, ...(node.outputs.length > 1 ? { label: id } : {}) })),
      position: node.position,
    };
    if (node.kind === 'trigger') return { ...base, kind: 'trigger', icon: 'webhook' };
    if (node.kind === 'control') return { ...base, kind: 'logic', logicType: 'logic' in node && node.logic === 'merge' ? 'merge' : 'if' };
    return { ...base, kind: 'action', icon: ('icon' in node ? node.icon : 'globe') as ReferenceIcon };
  });
  const edges: GraphEdge[] = reference.edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    sourceOutput: edge.sourceOutput,
    target: edge.target,
    targetInput: edge.targetInput,
    ...('loopBack' in edge ? { loopBack: true } : {}),
  }));
  return { nodes, edges };
}
