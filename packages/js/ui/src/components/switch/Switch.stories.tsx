// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Switch } from './Switch';
import { states } from './Switch.meta';

const meta = {
  title: 'Components/Switch',
  component: Switch,
  args: { label: 'Flow is enabled' },
} satisfies Meta<typeof Switch>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const toggle = within(canvasElement).getByRole('switch', { name: 'Flow is enabled' });
    await expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);
    await expect(toggle).toBeChecked();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Checked: Story = { args: { defaultChecked: true } };
export const Disabled: Story = { args: { disabled: true, defaultChecked: true } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  checked: { defaultChecked: true },
  disabled: { disabled: true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Switch states"
      columns={[...states]}
      rows={[{ label: 'switch', cells: states.map((state) => <Switch key={state} label="Label" {...stateProps[state]} />) }]}
    />
  ),
};
