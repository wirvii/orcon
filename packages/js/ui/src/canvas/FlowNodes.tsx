// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Node, NodeProps } from '@xyflow/react';

import { ControlNode } from './ControlNode';
import type { GraphNode } from './model';
import { NodeCard } from './NodeCard';

export interface FlowNodeData extends Record<string, unknown> {
  node: GraphNode;
  /** Ports accept pointer connections. */
  connectable: boolean;
}

export type OrconFlowNode = Node<FlowNodeData, 'action' | 'control'>;

/** React Flow renders these inside the focusable node element, which carries the accessible name. */
export function ActionFlowNode({ data, selected }: NodeProps<OrconFlowNode>) {
  return <NodeCard node={data.node} selected={selected} live connectable={data.connectable} labelled={false} />;
}

export function ControlFlowNode({ data, selected }: NodeProps<OrconFlowNode>) {
  return <ControlNode node={data.node} selected={selected} live connectable={data.connectable} labelled={false} />;
}

export const nodeTypes = { action: ActionFlowNode, control: ControlFlowNode };
