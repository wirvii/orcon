// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';

import { Combobox, type ComboboxOption } from './Combobox';
import { states } from './Combobox.meta';

const options: ComboboxOption[] = [
  { value: 'slack', label: 'Slack' },
  { value: 'gmail', label: 'Gmail', keywords: ['email', 'google'] },
  { value: 'sheets', label: 'Google Sheets', keywords: ['spreadsheet'] },
  { value: 'stripe', label: 'Stripe' },
  { value: 'github', label: 'GitHub', disabled: true },
];

const meta = {
  title: 'Components/Combobox',
  component: Combobox,
  args: { options, placeholder: 'Choose a connector', searchLabel: 'Search connectors' },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="h-panel-width max-w-screen-sm">
          <FormField label="Connector">
            <Story />
          </FormField>
        </div>
      ),
  ],
} satisfies Meta<typeof Combobox>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const trigger = within(canvasElement).getByRole('combobox', { name: 'Connector' });
    await userEvent.click(trigger);
    const search = await within(document.body).findByRole('combobox', { name: 'Search connectors' });
    // Typing filters the list, by label and by keywords.
    await userEvent.type(search, 'email');
    await expect(await within(document.body).findByRole('option', { name: 'Gmail' })).toBeInTheDocument();
    await expect(within(document.body).queryByRole('option', { name: 'Stripe' })).toBeNull();
    await userEvent.keyboard('{Enter}');
    await expect(trigger).toHaveTextContent('Gmail');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Open: Story = { args: { defaultOpen: true } };
export const Selected: Story = { args: { defaultValue: 'slack' } };
export const Disabled: Story = { args: { disabled: true, defaultValue: 'slack' } };
export const Error: Story = { args: { 'aria-invalid': true } };

const stateProps: Record<(typeof states)[number], Partial<React.ComponentProps<typeof Combobox>>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  open: {},
  selected: { defaultValue: 'slack' },
  disabled: { disabled: true, defaultValue: 'slack' },
  error: { 'aria-invalid': true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Combobox states (the open state is its own story)"
      columns={states.filter((state) => state !== 'open')}
      rows={[
        {
          label: 'combobox',
          cells: states
            .filter((state) => state !== 'open')
            .map((state) => (
              <div key={state} className="w-panel-width">
                <Combobox options={options} placeholder="Choose" searchLabel="Search" aria-label={`Connector, ${state}`} {...stateProps[state]} />
              </div>
            )),
        },
      ]}
    />
  ),
};
