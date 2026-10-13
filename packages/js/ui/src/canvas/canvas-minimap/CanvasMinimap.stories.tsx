// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Background, BackgroundVariant, ReactFlow, ReactFlowProvider } from '@xyflow/react';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import { exampleDraft } from '../fixtures';
import { nodeTypes, type TileFlowNode } from '../FlowNodes';

import { CanvasMinimap, type CanvasMinimapProps } from './CanvasMinimap';

const nodes: TileFlowNode[] = exampleDraft.nodes.map((node) => ({ id: node.id, type: 'tile', position: node.position, data: { node }, draggable: false }));

function Preview(props: CanvasMinimapProps) {
  return (
    <ReactFlowProvider>
      <div className="bg-canvas h-canvas-palette-w w-screen max-w-screen-sm overflow-hidden rounded-lg">
        <ReactFlow nodes={nodes} edges={[]} nodeTypes={nodeTypes} fitView nodesDraggable={false} nodesConnectable={false} nodesFocusable={false} elementsSelectable={false}>
          <Background variant={BackgroundVariant.Dots} gap={20} size={1.4} color="var(--orcon-canvas-grid)" />
          <CanvasMinimap {...props} />
        </ReactFlow>
      </div>
    </ReactFlowProvider>
  );
}

const meta = {
  title: 'Canvas/CanvasMinimap',
  component: CanvasMinimap,
  render: (args) => <Preview {...args} />,
} satisfies Meta<typeof CanvasMinimap>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { visible: true } };
export const Hidden: Story = { args: { visible: false } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Minimap"
      columns={['visible', 'hidden']}
      rows={[{ label: 'overview', cells: [<Preview key="v" visible />, <Preview key="h" visible={false} />] }]}
    />
  ),
};
