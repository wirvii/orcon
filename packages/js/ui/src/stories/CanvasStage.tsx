// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Background, BackgroundVariant, ReactFlow, ReactFlowProvider } from '@xyflow/react';
import type { ReactNode } from 'react';

import { CanvasEdge, type CanvasEdgeData, type CanvasFlowEdge } from '../canvas/canvas-edge/CanvasEdge';
import { exampleNodes } from '../canvas/fixtures';
import { nodeTypes, type TileFlowNode } from '../canvas/FlowNodes';
import type { GraphNode } from '../canvas/model';
import { cn } from '../lib/cn';

/** The canvas background around something drawn on it, so stories show the real contrast. Leaves room under it for the label. */
export function CanvasStage({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('bg-canvas inline-block rounded-lg px-8 pt-12 pb-16', className)}>{children}</div>;
}

const edgeTypes = { canvas: CanvasEdge };

/** Two nodes joined by one connection, drawn by React Flow, to show an edge the way the canvas draws it. */
export function EdgePreview({ data, loopBack = false }: { data?: CanvasEdgeData; loopBack?: boolean }) {
  const early: GraphNode = { ...exampleNodes.http, id: 'early', name: 'HTTP Request', status: undefined, position: { x: 0, y: 0 } };
  const late: GraphNode = { ...exampleNodes.slack, id: 'late', name: 'Slack', status: undefined, position: { x: 300, y: 0 } };
  const left = loopBack ? late : early;
  const right = loopBack ? early : late;
  const nodes: TileFlowNode[] = [
    { id: left.id, type: 'tile', position: { x: 0, y: 40 }, data: { node: { ...left, position: { x: 0, y: 40 } } }, draggable: false },
    { id: right.id, type: 'tile', position: { x: 300, y: 40 }, data: { node: { ...right, position: { x: 300, y: 40 } } }, draggable: false },
  ];
  const edges: CanvasFlowEdge[] = [
    {
      id: 'preview',
      type: 'canvas',
      source: loopBack ? 'late' : 'early',
      target: loopBack ? 'early' : 'late',
      sourceHandle: 'main',
      targetHandle: 'main',
      data: { ...data, loopBack, sourceName: 'HTTP Request', targetName: 'Slack', editable: true, detail: 'full' },
    },
  ];
  return (
    <ReactFlowProvider>
      <div className="bg-canvas h-panel-width w-screen max-w-screen-sm overflow-hidden rounded-lg">
        <ReactFlow<TileFlowNode, CanvasFlowEdge>
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
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
          <Background variant={BackgroundVariant.Dots} gap={20} size={1.4} color="var(--orcon-canvas-grid)" />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
}
