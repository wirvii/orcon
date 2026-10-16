// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ActiveToggle } from './ActiveToggle';
import { states } from './ActiveToggle.meta';

const meta = {
  title: 'Components/ActiveToggle',
  component: ActiveToggle,
  args: { checked: false, label: 'Sales agent', onCheckedChange: fn() },
} satisfies Meta<typeof ActiveToggle>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement, args }) => {
    const toggle = within(canvasElement).getByRole('switch', { name: 'Sales agent' });
    await expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);
    // It asks the server and does not move on its own.
    await expect(args.onCheckedChange).toHaveBeenCalledWith(true);
    await expect(toggle).not.toBeChecked();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Checked: Story = { args: { checked: true } };
export const Busy: Story = { args: { checked: false, busy: true } };
export const Blocked: Story = { args: { disabledReason: 'Publish the flow to be able to activate it' } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  checked: { checked: true },
  busy: { busy: true },
  blocked: { disabledReason: 'Publish the flow to be able to activate it' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ActiveToggle states"
      columns={[...states]}
      rows={[
        {
          label: 'toggle',
          cells: states.map((state) => <ActiveToggle key={state} checked={false} label={`Flow ${state}`} onCheckedChange={() => undefined} {...stateProps[state]} />),
        },
      ]}
    />
  ),
};
