// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { FlowListScreen } from './FlowListScreen';
import { must } from '../lib/must';

const meta = {
  title: 'Screens/S2 Flow list',
  component: FlowListScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof FlowListScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Loading: Story = { args: { status: 'loading' } };
export const Empty: Story = { args: { flows: [] } };
export const Error: Story = { args: { status: 'error' } };
export const Enabling: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // On a wide viewport the table is the visible control; on a phone the card list is. Either way the
    // switch of a disabled flow turns on and the change is announced.
    const switches = view.getAllByRole('switch', { name: 'Disabled: Welcome email' });
    const visible = switches.find((element) => element.getClientRects().length > 0) ?? must(switches[0]);
    await userEvent.click(visible);
    await expect(view.getByRole('status')).toHaveTextContent('Welcome email is now enabled');
  },
};

export const Matrix: Story = {};
