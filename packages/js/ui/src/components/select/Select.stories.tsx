// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';

import { Select } from './Select';
import { states } from './Select.meta';

const options = [
  { value: 'queued', label: 'Queued' },
  { value: 'immediate', label: 'Immediate' },
  { value: 'scheduled', label: 'Scheduled', disabled: true },
] as const;

const meta = {
  title: 'Components/Select',
  component: Select,
  args: { options, placeholder: 'Choose a run mode' },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
      <div className="max-w-screen-sm">
        <FormField label="Run mode">
          <Story />
        </FormField>
      </div>
    ),
  ],
} satisfies Meta<typeof Select>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const trigger = within(canvasElement).getByRole('combobox', { name: 'Run mode' });
    await userEvent.click(trigger);
    const option = await within(document.body).findByRole('option', { name: 'Immediate' });
    await userEvent.click(option);
    await expect(trigger).toHaveTextContent('Immediate');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Open: Story = {
  args: { defaultOpen: true, defaultValue: 'queued' },
  decorators: [
    (Story) => (
      <div className="h-panel-width">
        <Story />
      </div>
    ),
  ],
  parameters: {
    // While the listbox is open, Radix hides the rest of the page from assistive technology and
    // keeps focus inside the listbox. The trigger stays in the DOM and axe flags it as hidden but
    // focusable. People cannot reach it until the listbox closes, so this is a false positive.
    a11y: { config: { rules: [{ id: 'aria-hidden-focus', enabled: false }] } },
  },
};
export const Disabled: Story = { args: { disabled: true, defaultValue: 'queued' } };
export const Error: Story = { args: { 'aria-invalid': true } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  open: {},
  disabled: { disabled: true, defaultValue: 'queued' },
  error: { 'aria-invalid': true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Select states (the open state is its own story)"
      columns={states.filter((state) => state !== 'open')}
      rows={[
        {
          label: 'select',
          cells: states
            .filter((state) => state !== 'open')
            .map((state) => (
              <div key={state} className="w-panel-width">
                <Select options={options} aria-label={`Run mode, ${state}`} placeholder="Choose" {...stateProps[state]} />
              </div>
            )),
        },
      ]}
    />
  ),
};
