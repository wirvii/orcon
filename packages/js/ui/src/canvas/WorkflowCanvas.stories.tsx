// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, waitFor, within } from 'storybook/test';

import { Toaster } from '../components/toast/Toast';
import { MatrixTable } from '../stories/MatrixTable';

import { agentGraph, exampleCatalog, exampleDraft, exampleGraph, largeGraph, placeholderNode } from './fixtures';
import type { Graph } from './model';
import { WorkflowCanvas } from './WorkflowCanvas';
import { must } from '../lib/must';

const meta = {
  title: 'Canvas/WorkflowCanvas',
  component: WorkflowCanvas,
  args: { defaultGraph: exampleDraft, catalog: exampleCatalog, tools: true },
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
export const Empty: Story = {
  args: { defaultGraph: { nodes: [], edges: [] } },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // Editing needs md or wider; on a phone the empty canvas says so instead.
    if (view.queryByRole('button', { name: 'Add node' })) await expect(view.getByRole('button', { name: 'Add first step' })).toBeVisible();
    else await expect(view.getByText('This flow is empty')).toBeVisible();
  },
};
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
  args: { defaultGraph: exampleGraph, readOnly: true, tools: false, executionHref: (id) => `/executions/example/${id}` },
};
export const Agent: Story = { args: { defaultGraph: agentGraph } };
export const Compare: Story = {
  args: {
    readOnly: true,
    tools: false,
    defaultGraph: {
      ...exampleDraft,
      nodes: [
        ...exampleDraft.nodes.map((node) => (node.id === 'http' ? { ...node, diff: 'changed' as const } : node)),
        { ...placeholderNode, id: 'unsupported', position: { x: 440, y: 440 } },
        { ...must(exampleDraft.nodes[0]), id: 'added', name: 'Audit log', subtitle: 'Write entry', diff: 'added' as const, kind: 'action' as const, icon: 'file' as const, inputs: [{ id: 'main' }], position: { x: 640, y: 280 } },
        { ...must(exampleDraft.nodes[0]), id: 'removed', name: 'Notify team', subtitle: 'Send message', diff: 'removed' as const, kind: 'action' as const, icon: 'message' as const, inputs: [{ id: 'main' }], position: { x: 840, y: 40 } },
      ],
    } satisfies Graph,
  },
};
/** 500 nodes (SPEC-010 PF-4): only the nodes on screen are in the page, and the level of detail drops with the zoom. */
export const Performance: Story = {
  args: { defaultGraph: largeGraph(500), tools: false },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('region', { name: /Flow canvas: 500 nodes/ })).toBeVisible();
  },
};
export const Keyboard: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // Phones are view only: there is no toolbar to reach.
    if (!view.queryByRole('button', { name: 'Add node' })) return;
    (await view.findByRole('group', { name: /^Webhook, / })).focus();
    // One node holds the tab stop and shows its toolbar.
    await waitFor(() => expect(view.getAllByRole('toolbar', { name: /Actions for/ }).length).toBe(1));
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
              <WorkflowCanvas defaultGraph={exampleDraft} catalog={exampleCatalog} tools />
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
        {
          label: 'agent',
          cells: [
            <div key="c" className="h-screen w-screen max-w-screen-md">
              <WorkflowCanvas defaultGraph={agentGraph} catalog={exampleCatalog} />
            </div>,
          ],
        },
      ]}
    />
  ),
};
