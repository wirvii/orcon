// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { CanvasStage } from '../stories/CanvasStage';
import { MatrixTable } from '../stories/MatrixTable';

import { ControlNode } from './ControlNode';
import { exampleNodes } from './fixtures';
import type { ControlType, GraphNode } from './model';

const main = [{ id: 'main' }];

function control(type: ControlType, name: string, outputs: GraphNode['outputs'] = main, inputs: GraphNode['inputs'] = main): GraphNode {
  return { id: `control-${type}`, kind: 'control', controlType: type, name, inputs, outputs, position: { x: 0, y: 0 } };
}

const nodes: Record<ControlType, GraphNode> = {
  if: { ...exampleNodes.branch, status: undefined },
  switch: control('switch', 'Shipping method', [
    { id: 'standard', label: 'standard' },
    { id: 'express', label: 'express' },
    { id: 'pickup', label: 'pickup' },
  ]),
  merge: control('merge', 'Merge paths', main, [
    { id: 'a', label: 'a' },
    { id: 'b', label: 'b' },
  ]),
  loop: control('loop', 'Each product'),
  wait: control('wait', 'Wait for payment'),
};

const meta = {
  title: 'Canvas/ControlNode',
  component: ControlNode,
  args: { node: nodes.if },
  decorators: [(Story) => <CanvasStage><Story /></CanvasStage>],
} satisfies Meta<typeof ControlNode>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('group')).toHaveAccessibleName(/^Control node In stock\?/);
    await expect(within(canvasElement).getByText('true')).toBeVisible();
    await expect(within(canvasElement).getByText('false')).toBeVisible();
  },
};
export const If: Story = { args: { node: nodes.if } };
export const Switch: Story = { args: { node: nodes.switch } };
export const Merge: Story = { args: { node: nodes.merge } };
export const Loop: Story = { args: { node: nodes.loop } };
export const Wait: Story = { args: { node: nodes.wait } };
export const Selected: Story = { args: { selected: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Control nodes"
      columns={['plain', 'selected']}
      rows={(Object.keys(nodes) as ControlType[]).map((type) => ({
        label: type,
        cells: [false, true].map((selected) => (
          <CanvasStage key={String(selected)}>
            <ControlNode node={nodes[type]} selected={selected} />
          </CanvasStage>
        )),
      }))}
    />
  ),
};
