// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { AttentionInboxScreen } from './AttentionInboxScreen';

const meta = {
  title: 'Screens/S6 Attention inbox',
  component: AttentionInboxScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof AttentionInboxScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Empty: Story = { args: { items: [] } };
export const Error: Story = { args: { status: 'error' } };
export const BatchSelected: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // Two approvals of the same flow can be acknowledged together; adding another cause turns the action off.
    await userEvent.click(view.getByRole('checkbox', { name: 'Select Approve a refund of 480 USD' }));
    await userEvent.click(view.getByRole('checkbox', { name: 'Select Approve a refund of 120 USD' }));
    await expect(view.getByRole('button', { name: 'Acknowledge selected' })).toBeEnabled();
    await userEvent.click(view.getByRole('checkbox', { name: 'Select Review a card charge that may have been applied' }));
    await expect(view.getByRole('button', { name: 'Acknowledge selected' })).toBeDisabled();
  },
};

export const Matrix: Story = {};
