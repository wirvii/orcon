// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';
import { Input } from '../input/Input';

import { ErrorSummary } from './ErrorSummary';
import { states } from './ErrorSummary.meta';

const errors = [
  { fieldId: 'flow-name', message: 'Enter a name for the flow.' },
  { fieldId: 'flow-owner', message: 'Choose an owner.' },
] as const;

const meta = {
  title: 'Components/ErrorSummary',
  component: ErrorSummary,
  args: { errors },
  render: (args) => (
    <div className="flex max-w-screen-sm flex-col gap-4">
      <ErrorSummary {...args} />
      <FormField id="flow-name" label="Flow name" error="Enter a name for the flow.">
        <Input />
      </FormField>
      <FormField id="flow-owner" label="Owner" error="Choose an owner.">
        <Input />
      </FormField>
    </div>
  ),
} satisfies Meta<typeof ErrorSummary>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('link', { name: 'Choose an owner.' }));
    // Following a link in the summary moves focus to the field.
    await expect(canvas.getByLabelText(/Owner/)).toHaveFocus();
  },
};
export const FocusVisible: Story = { args: { focusOnMount: true } };
export const Single: Story = { args: { errors: [errors[0]] } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ErrorSummary states"
      columns={['default', 'single']}
      rows={[
        {
          label: 'summary',
          cells: [
            <div key="d" className="w-panel-width">
              <ErrorSummary errors={errors} />
            </div>,
            <div key="s" className="w-panel-width">
              <ErrorSummary errors={[errors[0]]} />
            </div>,
          ],
        },
      ]}
    />
  ),
};

void states;
