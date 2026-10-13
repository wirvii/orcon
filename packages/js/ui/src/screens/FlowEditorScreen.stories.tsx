// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { exampleGraph } from '../canvas/fixtures';
import { orderItems } from '../editor/fixtures';

import { FlowEditorScreen } from './FlowEditorScreen';

const meta = {
  title: 'Screens/S3 Flow editor',
  component: FlowEditorScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof FlowEditorScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('heading', { level: 1, name: 'Order intake' })).toBeVisible();
  },
};
export const Empty: Story = { args: { graph: { nodes: [], edges: [] } } };
export const ViewOnly: Story = { args: { graph: exampleGraph, readOnly: true, executionHref: (id) => `#executions/ex-4821/${id}` } };
export const Execution: Story = {
  args: {
    graph: exampleGraph,
    outputs: { http: { main: orderItems }, validate: { true: orderItems.slice(0, 2), false: orderItems.slice(2) } },
    executionHref: (id) => `#executions/ex-4821/${id}`,
  },
};
/** The node details view: three panes from lg, the docked panel and the bottom data panel below it. */
export const Details: Story = {
  args: {
    graph: exampleGraph,
    defaultDetailsId: 'slack',
    outputs: { webhook: { main: orderItems }, http: { main: orderItems }, slack: { main: orderItems } },
  },
};

export const Matrix: Story = {};
