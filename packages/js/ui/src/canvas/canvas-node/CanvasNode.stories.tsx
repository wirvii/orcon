// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { must } from '../../lib/must';
import { CanvasStage } from '../../stories/CanvasStage';
import { agentGraph, exampleNodes, placeholderNode } from '../fixtures';
import type { GraphNode } from '../model';

import { CanvasNode, type CanvasNodeProps } from './CanvasNode';
import type { states } from './CanvasNode.meta';

const meta = {
  title: 'Canvas/CanvasNode',
  component: CanvasNode,
  args: { node: { ...exampleNodes.http, status: undefined }, labelled: true, editable: false },
  decorators: [
    (Story) => (
      <CanvasStage>
        <Story />
      </CanvasStage>
    ),
  ],
} satisfies Meta<typeof CanvasNode>;

export default meta;
type Story = StoryObj<typeof meta>;

const plain: GraphNode = { ...exampleNodes.http, status: undefined };
const trigger: GraphNode = { ...exampleNodes.webhook, status: undefined };
const logic: GraphNode = { ...exampleNodes.validate, status: undefined };
const agent: GraphNode = { ...must(agentGraph.nodes[1]) };
const connector: GraphNode = { ...exampleNodes.slack, status: undefined };

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('group', { name: 'HTTP Request, GET inventory. Status: Not executed. 1 input, 1 output.' })).toBeVisible();
  },
};
export const Trigger: Story = { args: { node: trigger } };
export const Logic: Story = { args: { node: logic } };
export const Agent: Story = { args: { node: agent } };
export const Placeholder: Story = { args: { node: placeholderNode } };
export const Selected: Story = { args: { selected: true } };
export const Hover: Story = { args: { className: 'pseudo-hover', forceToolbar: true, editable: true } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible', forceToolbar: true, editable: true } };
export const Running: Story = { args: { node: { ...plain, status: 'RUNNING' } } };
export const Succeeded: Story = { args: { node: { ...plain, status: 'SUCCEEDED' } } };
export const SucceededWithErrors: Story = { args: { node: { ...plain, status: 'SUCCEEDED_WITH_ERRORS' } } };
export const Failed: Story = { args: { node: { ...connector, status: 'FAILED', statusCode: '401' } } };
export const Uncertain: Story = { args: { node: { ...connector, status: 'UNCERTAIN' } } };
export const Waiting: Story = { args: { node: { ...plain, status: 'WAITING' } } };
export const Canceled: Story = { args: { node: { ...plain, status: 'CANCELED' } } };
export const NotRun: Story = { args: { node: { ...plain, status: 'NOT_RUN' } } };
export const Disabled: Story = { args: { node: { ...plain, disabled: true } } };
export const Pinned: Story = { args: { node: { ...plain, status: 'SUCCEEDED', pinned: true } } };
export const NeverSendsTwice: Story = {
  args: { node: { ...connector, neverTwice: true } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Never sends twice')).toBeVisible();
  },
};
export const Compare: Story = { args: { node: { ...plain, diff: 'changed' } } };

const gallery: Record<(typeof states)[number], CanvasNodeProps> = {
  default: { node: plain },
  trigger: { node: trigger },
  logic: { node: logic },
  agent: { node: agent },
  placeholder: { node: placeholderNode },
  selected: { node: plain, selected: true },
  hover: { node: plain, className: 'pseudo-hover', editable: true },
  'focus-visible': { node: plain, className: 'pseudo-focus-visible', editable: true },
  running: { node: { ...plain, status: 'RUNNING' } },
  succeeded: { node: { ...plain, status: 'SUCCEEDED' } },
  'succeeded-with-errors': { node: { ...plain, status: 'SUCCEEDED_WITH_ERRORS' } },
  failed: { node: { ...connector, status: 'FAILED', statusCode: '401' } },
  uncertain: { node: { ...connector, status: 'UNCERTAIN' } },
  waiting: { node: { ...plain, status: 'WAITING' } },
  canceled: { node: { ...plain, status: 'CANCELED' } },
  'not-run': { node: { ...plain, status: 'NOT_RUN' } },
  disabled: { node: { ...plain, disabled: true } },
  pinned: { node: { ...plain, status: 'SUCCEEDED', pinned: true } },
  compare: { node: { ...plain, diff: 'changed' } },
};

/** Every state of a node, to review and to compare with the style A mockup. */
export const Matrix: Story = {
  decorators: [],
  render: () => (
    <div className="bg-canvas grid grid-cols-2 gap-x-16 gap-y-8 rounded-lg px-16 pt-16 pb-16 sm:grid-cols-3 xl:grid-cols-5">
      {(Object.keys(gallery) as (keyof typeof gallery)[]).map((state) => (
        <div key={state} data-matrix-cell={state} className="flex justify-center pb-16">
          <CanvasNode labelled editable={false} {...gallery[state]} node={{ ...gallery[state].node, id: `${gallery[state].node.id}-${state}` }} />
        </div>
      ))}
    </div>
  ),
};
