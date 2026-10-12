// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { EmptyStatesScreen } from './EmptyStatesScreen';

const meta = {
  title: 'Screens/S8 Empty states',
  component: EmptyStatesScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof EmptyStatesScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // Only the first-time empty state carries the mascot placeholder; every empty state says what to do.
    await expect(canvasElement.querySelectorAll('[data-mascot-pose]')).toHaveLength(1);
    await expect(within(canvasElement).getByRole('button', { name: 'New flow' })).toBeVisible();
  },
};

export const Matrix: Story = {};
