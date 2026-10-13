// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';

import { Input } from './Input';
import { states } from './Input.meta';

const meta = {
  title: 'Components/Input',
  component: Input,
  args: { placeholder: 'Untitled flow' },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
      <div className="max-w-screen-sm">
        <FormField label="Flow name">
          <Story />
        </FormField>
      </div>
    ),
  ],
} satisfies Meta<typeof Input>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByLabelText('Flow name');
    await userEvent.type(input, 'Nightly sync');
    await expect(input).toHaveValue('Nightly sync');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Disabled: Story = { args: { disabled: true, value: 'Nightly sync' } };
export const Error: Story = { args: { 'aria-invalid': true, defaultValue: '' } };

const stateProps: Record<(typeof states)[number], { className?: string; disabled?: boolean; 'aria-invalid'?: boolean }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  disabled: { disabled: true },
  error: { 'aria-invalid': true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Input states"
      columns={[...states]}
      rows={[
        {
          label: 'text',
          cells: states.map((state) => (
            <div key={state} className="w-panel-width">
              <Input aria-label={`Input, ${state}`} placeholder="Placeholder" defaultValue={state === 'disabled' ? 'Value' : undefined} {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
