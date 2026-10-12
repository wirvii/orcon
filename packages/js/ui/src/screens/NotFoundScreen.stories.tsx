// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { NotFoundScreen } from './NotFoundScreen';

const meta = {
  title: 'Screens/S8 Not found',
  component: NotFoundScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof NotFoundScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('[data-mascot-pose="explorer"]')).not.toBeNull();
    await expect(within(canvasElement).getByRole('heading', { level: 1, name: 'Page not found' })).toBeVisible();
  },
};

export const Matrix: Story = {};
