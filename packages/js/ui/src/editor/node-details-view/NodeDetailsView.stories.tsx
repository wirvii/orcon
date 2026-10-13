// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { agentGraph, exampleNodes } from '../../canvas/fixtures';
import { DetailsDemo } from '../sample/DetailsDemo';
import { must } from '../../lib/must';


const slack = { ...exampleNodes.slack, status: undefined, statusCode: undefined };

const meta = {
  title: 'Editor/NodeDetailsView',
  // The view is a dialog over the page, so the stories show it open.
  component: DetailsDemo,
  parameters: { layout: 'fullscreen' },
  args: { node: slack, layout: 'dialog' },
} satisfies Meta<typeof DetailsDemo>;

export default meta;
type Story = StoryObj<typeof meta>;

const body = (canvasElement: HTMLElement) => within(canvasElement.ownerDocument.body);

export const Default: Story = {
  args: {},
  play: async ({ canvasElement }) => {
    const dialog = await body(canvasElement).findByRole('dialog', { name: 'Slack details' });
    await expect(dialog).toBeVisible();
    // Three panes: Input | Parameters | Output.
    for (const pane of ['input', 'parameters', 'output']) await expect(dialog.querySelector(`[data-pane="${pane}"]`)).not.toBeNull();
    await expect(within(dialog).getByRole('button', { name: 'Execute step' })).toBeVisible();
    await expect(within(dialog).getByRole('button', { name: 'Back to canvas' })).toBeVisible();
    await expect(within(dialog).getAllByRole('separator')).toHaveLength(2);
  },
};
export const Trigger: Story = {
  args: { node: { ...exampleNodes.webhook, status: undefined } },
  play: async ({ canvasElement }) => {
    const dialog = await body(canvasElement).findByRole('dialog', { name: 'Webhook details' });
    // A trigger has no Input pane.
    await expect(dialog.querySelector('[data-pane="input"]')).toBeNull();
    await expect(within(dialog).getAllByRole('separator')).toHaveLength(1);
  },
};
export const Agent: Story = {
  args: { node: must(agentGraph.nodes[1]) },
  play: async ({ canvasElement }) => {
    const dialog = await body(canvasElement).findByRole('dialog', { name: 'Support agent details' });
    await expect(within(dialog).getByRole('tab', { name: 'Agent' })).toBeVisible();
  },
};
export const Failed: Story = {
  args: { node: exampleNodes.gmail },
  play: async ({ canvasElement }) => {
    const dialog = await body(canvasElement).findByRole('dialog', { name: 'Gmail details' });
    await expect(within(dialog).getByText('The step failed')).toBeVisible();
    await expect(within(dialog).getByRole('button', { name: 'Reconnect credential' })).toBeVisible();
  },
};
export const Uncertain: Story = {
  args: { node: exampleNodes.slack },
  play: async ({ canvasElement }) => {
    const dialog = await body(canvasElement).findByRole('dialog', { name: 'Slack details' });
    await expect(within(dialog).getByText('Outcome unknown')).toBeVisible();
    // Running again is never the primary action of an uncertain node.
    const run = within(dialog).getByRole('button', { name: 'Run again' });
    await expect(run.className).not.toContain('bg-action-primary');
  },
};
export const Docked: Story = {
  args: { layout: 'docked' },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('complementary', { name: 'Slack details' })).toBeVisible();
  },
};
export const Tablet: Story = {
  // From the lg breakpoint it is a dialog, below it the docked panel: the layout follows the width.
  args: { layout: 'auto' },
};

export const Matrix: Story = {
  args: { layout: 'auto' },
};
