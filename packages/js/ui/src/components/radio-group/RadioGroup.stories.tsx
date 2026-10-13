// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { RadioGroup, RadioGroupItem } from './RadioGroup';
import { states } from './RadioGroup.meta';

const meta = {
  title: 'Components/RadioGroup',
  component: RadioGroup,
  args: { 'aria-label': 'Run mode' },
  render: (args) => (
    <RadioGroup {...args}>
      <RadioGroupItem value="queued" label="Queued" />
      <RadioGroupItem value="immediate" label="Immediate" />
      <RadioGroupItem value="scheduled" label="Scheduled" />
    </RadioGroup>
  ),
} satisfies Meta<typeof RadioGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('radio', { name: 'Immediate' }));
    await expect(canvas.getByRole('radio', { name: 'Immediate' })).toBeChecked();
    await expect(canvas.getByRole('radio', { name: 'Queued' })).not.toBeChecked();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Selected: Story = { args: { defaultValue: 'queued' } };
export const Disabled: Story = { args: { disabled: true, defaultValue: 'queued' } };
export const Error: Story = { args: { 'aria-invalid': true } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  selected: { defaultValue: 'a' },
  disabled: { disabled: true, defaultValue: 'a' },
  error: { 'aria-invalid': true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="RadioGroup states"
      columns={[...states]}
      rows={[
        {
          label: 'radio group',
          cells: states.map((state) => (
            <RadioGroup key={state} aria-label={`Options, ${state}`} {...stateProps[state]}>
              <RadioGroupItem value="a" label="Option A" />
              <RadioGroupItem value="b" label="Option B" />
            </RadioGroup>
          )),
        },
      ]}
    />
  ),
};
