// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Background, BackgroundVariant, ReactFlow, ReactFlowProvider } from '@xyflow/react';
import type { ReactNode } from 'react';

import { exampleNodes } from '../canvas/fixtures';
import { OrconEdge, type OrconEdgeData, type OrconFlowEdge } from '../canvas/Edge';
import { nodeTypes, type OrconFlowNode } from '../canvas/FlowNodes';
import { cn } from '../lib/cn';

/** The canvas background around something drawn on it, so stories show the real contrast. */
export function CanvasStage({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('bg-canvas inline-block rounded-lg p-8', className)}>{children}</div>;
}

const edgeTypes = { orcon: OrconEdge };

/** Two nodes joined by one connection, drawn by React Flow, to show an edge the way the canvas draws it. */
export function EdgePreview({ data, selected = false, loopBack = false }: { data?: OrconEdgeData; selected?: boolean; loopBack?: boolean }) {
  const first = { ...exampleNodes.check, position: { x: 0, y: loopBack ? 0 : 0 } };
  const second = { ...exampleNodes.charge, status: undefined, uncertainCount: undefined, position: { x: loopBack ? 0 : 320, y: loopBack ? 0 : 0 } };
  const nodes: OrconFlowNode[] = loopBack
    ? [
        { id: 'late', type: 'action', position: { x: 320, y: 0 }, data: { node: { ...second, id: 'late', name: 'Create back order', position: { x: 320, y: 0 } }, connectable: false }, draggable: false },
        { id: 'early', type: 'action', position: { x: 0, y: 0 }, data: { node: { ...first, id: 'early', status: undefined }, connectable: false }, draggable: false },
      ]
    : [
        { id: 'early', type: 'action', position: { x: 0, y: 0 }, data: { node: { ...first, id: 'early', status: undefined }, connectable: false }, draggable: false },
        { id: 'late', type: 'action', position: { x: 320, y: 0 }, data: { node: { ...second, id: 'late' }, connectable: false }, draggable: false },
      ];
  const edges: OrconFlowEdge[] = [
    {
      id: 'preview',
      type: 'orcon',
      source: loopBack ? 'late' : 'early',
      target: loopBack ? 'early' : 'late',
      sourceHandle: 'main',
      targetHandle: 'main',
      selected,
      data: { ...data, loopBack },
    },
  ];
  return (
    <ReactFlowProvider>
      <div className="bg-canvas h-node-width w-full overflow-hidden rounded-lg">
        <ReactFlow<OrconFlowNode, OrconFlowEdge>
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          fitView
          fitViewOptions={{ padding: 0.1 }}
          nodesDraggable={false}
          nodesConnectable={false}
          nodesFocusable={false}
          edgesFocusable={false}
          elementsSelectable={false}
          panOnDrag={false}
          zoomOnScroll={false}
          zoomOnPinch={false}
          zoomOnDoubleClick={false}
          preventScrolling={false}
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1.5} color="var(--orcon-canvas-grid)" />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
}
