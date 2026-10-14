// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { ErrorPageScreen } from './ErrorPageScreen';

const meta = {
  title: 'Screens/S8 Error page',
  component: ErrorPageScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof ErrorPageScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The full-page error screen shows the thinking mascot, and the request number is there to report.
    await expect(canvasElement.querySelector('[data-mascot-pose="thinking"]')).not.toBeNull();
    await expect(within(canvasElement).getByText(/req_5b21c9e0/)).toBeVisible();
  },
};

export const Matrix: Story = {};
