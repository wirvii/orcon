// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { executionTimeline } from './fixtures';
import { ExecutionDetailScreen } from './ExecutionDetailScreen';

const meta = {
  title: 'Screens/S5 Execution detail',
  component: ExecutionDetailScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof ExecutionDetailScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // The panel offers the resolutions it received, with no primary repeat; Continue waits for a decision.
    await expect(view.getByRole('button', { name: 'Continue' })).toBeDisabled();
    await expect(view.getByRole('button', { name: 'Charge again' })).toBeDisabled();
  },
};
export const Redacted: Story = {
  args: { canReadData: false },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getAllByText('Hidden — you need permission to view execution data').length).toBeGreaterThan(0);
    await expect(canvasElement.textContent).not.toContain('pay_7Q2K"');
    await expect(canvasElement.textContent).not.toContain('ana.ruiz@example.test');
  },
};
export const NoDecision: Story = {
  args: { timeline: executionTimeline.map((node) => (node.status === 'UNCERTAIN' ? { ...node, status: 'SUCCEEDED' as const } : node)), resolution: undefined },
};
export const Succeeded: Story = {
  args: {
    timeline: executionTimeline.map((node) => ({ ...node, status: 'SUCCEEDED' as const, attempts: node.attempts.map((attempt) => ({ ...attempt, status: 'SUCCEEDED' as const })) })),
    resolution: undefined,
  },
};

export const Matrix: Story = {};
