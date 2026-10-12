// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { ExecutionListScreen } from './ExecutionListScreen';

const meta = {
  title: 'Screens/S4 Execution list',
  component: ExecutionListScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof ExecutionListScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Loading: Story = { args: { status: 'loading' } };
export const Empty: Story = { args: { executions: [] } };
export const Error: Story = { args: { status: 'error' } };

export const Matrix: Story = {};
