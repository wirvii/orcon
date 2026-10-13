// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Checkbox } from './Checkbox';
import { states } from './Checkbox.meta';

const meta = {
  title: 'Components/Checkbox',
  component: Checkbox,
  args: { label: 'Retry failed items automatically' },
} satisfies Meta<typeof Checkbox>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const box = within(canvasElement).getByRole('checkbox', { name: 'Retry failed items automatically' });
    await expect(box).not.toBeChecked();
    // The label is part of the control, so clicking the text toggles it.
    await userEvent.click(within(canvasElement).getByText('Retry failed items automatically'));
    await expect(box).toBeChecked();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Checked: Story = { args: { defaultChecked: true } };
export const Indeterminate: Story = { args: { checked: 'indeterminate' } };
export const Disabled: Story = { args: { disabled: true, defaultChecked: true } };
export const Error: Story = { args: { 'aria-invalid': true } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  checked: { defaultChecked: true },
  indeterminate: { checked: 'indeterminate' },
  disabled: { disabled: true },
  error: { 'aria-invalid': true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Checkbox states"
      columns={[...states]}
      rows={[{ label: 'checkbox', cells: states.map((state) => <Checkbox key={state} label="Label" {...stateProps[state]} />) }]}
    />
  ),
};
