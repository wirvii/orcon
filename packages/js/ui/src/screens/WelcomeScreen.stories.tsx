// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { WelcomeScreen } from './WelcomeScreen';

const meta = {
  title: 'Screens/S1 Welcome',
  component: WelcomeScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof WelcomeScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('heading', { level: 1, name: 'Welcome to Orcon' })).toBeVisible();
    await expect(view.getByRole('button', { name: 'Create your first flow' })).toBeVisible();
  },
};

/** The template as a whole, for the visual and the owner review. */
export const Matrix: Story = {};
