// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { AppearanceSettingsScreen } from './AppearanceSettingsScreen';

const meta = {
  title: 'Screens/S9 Appearance settings',
  component: AppearanceSettingsScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof AppearanceSettingsScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('radiogroup', { name: 'Theme' })).toBeVisible();
    await expect(view.getByRole('radiogroup', { name: 'Density' })).toBeVisible();
  },
};

export const Matrix: Story = {};
