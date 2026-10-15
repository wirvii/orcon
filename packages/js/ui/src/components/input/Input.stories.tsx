// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Eye, Mail } from 'lucide-react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';
import { IconButton } from '../icon-button/IconButton';

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
/** An icon before the text; the label says what the field is, the icon is only a hint. */
export const WithIcon: Story = { args: { startIcon: Mail, placeholder: 'ana@example.com' } };
/** A control inside the field after the text, such as the button that shows a password. It is its own tab stop. */
export const WithAdornment: Story = {
  args: { endAdornment: <IconButton variant="ghost" icon={Eye} label="Show value" className="size-8" /> },
};

const stateProps: Record<(typeof states)[number], Partial<React.ComponentProps<typeof Input>>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  disabled: { disabled: true },
  error: { 'aria-invalid': true },
  'with-icon': { startIcon: Mail },
  'with-adornment': { endAdornment: <IconButton variant="ghost" icon={Eye} label="Show value" className="size-8" /> },
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
