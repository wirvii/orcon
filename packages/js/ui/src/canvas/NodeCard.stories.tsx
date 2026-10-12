// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { CanvasStage } from '../stories/CanvasStage';
import { MatrixTable } from '../stories/MatrixTable';

import { exampleNodes } from './fixtures';
import type { GraphNode } from './model';
import { NodeCard } from './NodeCard';

const plain: GraphNode = { ...exampleNodes.check, status: undefined };

const meta = {
  title: 'Canvas/NodeCard',
  component: NodeCard,
  args: { node: plain },
  decorators: [(Story) => <CanvasStage><Story /></CanvasStage>],
} satisfies Meta<typeof NodeCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The accessible name carries kind, name, summary, ports and no status.
    await expect(within(canvasElement).getByRole('group')).toHaveAccessibleName(
      'Action node Check stock, Looks up every product of the order in the warehouse list., 1 input, 1 output',
    );
  },
};
export const Trigger: Story = {
  args: { node: { ...exampleNodes.trigger, status: undefined } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('img', { name: 'Starts the flow' })).toBeVisible();
    await expect(within(canvasElement).getByRole('group')).toHaveAccessibleName(/^Trigger node Order received/);
  },
};
export const Selected: Story = { args: { selected: true } };
export const Running: Story = { args: { node: { ...plain, status: 'RUNNING' } } };
export const Succeeded: Story = { args: { node: { ...plain, status: 'SUCCEEDED' } } };
export const SucceededWithErrors: Story = { args: { node: { ...plain, status: 'SUCCEEDED_WITH_ERRORS' } } };
export const Failed: Story = { args: { node: { ...plain, status: 'FAILED' } } };
export const Uncertain: Story = { args: { node: { ...plain, status: 'UNCERTAIN' } } };
export const WithUncertainItems: Story = { args: { node: exampleNodes.charge } };
export const NotRun: Story = { args: { node: { ...plain, status: 'NOT_RUN' } } };
export const LabelledOutputs: Story = {
  args: {
    node: {
      ...plain,
      name: 'Validate address',
      summary: 'Checks the shipping address.',
      outputs: [
        { id: 'valid', label: 'valid' },
        { id: 'invalid', label: 'invalid' },
      ],
    },
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Action and trigger nodes"
      columns={['plain', 'selected', 'with state']}
      rows={[
        {
          label: 'action',
          cells: [
            <CanvasStage key="a"><NodeCard node={plain} /></CanvasStage>,
            <CanvasStage key="b"><NodeCard node={plain} selected /></CanvasStage>,
            <CanvasStage key="c"><NodeCard node={exampleNodes.charge} /></CanvasStage>,
          ],
        },
        {
          label: 'trigger',
          cells: [
            <CanvasStage key="a"><NodeCard node={{ ...exampleNodes.trigger, status: undefined }} /></CanvasStage>,
            <CanvasStage key="b"><NodeCard node={{ ...exampleNodes.trigger, status: undefined }} selected /></CanvasStage>,
            <CanvasStage key="c"><NodeCard node={exampleNodes.trigger} /></CanvasStage>,
          ],
        },
      ]}
    />
  ),
};
