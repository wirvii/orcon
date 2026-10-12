// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../stories/MatrixTable';
import { Toaster } from '../components/toast/Toast';

import { exampleCatalog, exampleDraft, exampleGraph } from './fixtures';
import { WorkflowCanvas } from './WorkflowCanvas';

const meta = {
  title: 'Canvas/WorkflowCanvas',
  component: WorkflowCanvas,
  args: { defaultGraph: exampleDraft, catalog: exampleCatalog },
  decorators: [
    (Story) => (
      <div className="h-screen">
        <Story />
        <Toaster />
      </div>
    ),
  ],
} satisfies Meta<typeof WorkflowCanvas>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Empty: Story = { args: { defaultGraph: { nodes: [], edges: [] } } };
export const ViewOnly: Story = {
  args: { readOnly: true, defaultGraph: exampleGraph, executionHref: (id) => `/executions/example/${id}` },
  play: async ({ canvasElement }) => {
    // No editing affordance is rendered, and nothing says the canvas can be edited.
    await expect(within(canvasElement).queryByRole('button', { name: 'Add node' })).toBeNull();
    await expect(within(canvasElement).getByText(/View only/)).toBeVisible();
  },
};
export const OutlineOpen: Story = {
  args: { defaultOutlineOpen: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('region', { name: 'Flow outline' })).toBeVisible();
  },
};
export const Execution: Story = {
  args: {
    defaultGraph: exampleGraph,
    statuses: { 'send-receipt': { status: 'WAITING' } },
    executionHref: (id) => `/executions/example/${id}`,
  },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // Tab enters the canvas on one node; Enter opens its details panel when the host gives no inspector.
    await userEvent.tab();
    await expect(view.getAllByRole('toolbar').length).toBe(1);
  },
};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Workflow canvas"
      columns={['canvas']}
      rows={[
        {
          label: 'editing',
          cells: [
            <div key="a" className="h-screen w-screen max-w-screen-md">
              <WorkflowCanvas defaultGraph={exampleDraft} catalog={exampleCatalog} />
            </div>,
          ],
        },
        {
          label: 'with an execution',
          cells: [
            <div key="b" className="h-screen w-screen max-w-screen-md">
              <WorkflowCanvas defaultGraph={exampleGraph} readOnly />
            </div>,
          ],
        },
      ]}
    />
  ),
};
