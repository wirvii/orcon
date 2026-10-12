// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import { FormField } from '../form-field/FormField';

import { Textarea } from './Textarea';
import { states } from './Textarea.meta';

const meta = {
  title: 'Components/Textarea',
  component: Textarea,
  args: { placeholder: 'What does this flow do?' },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
      <div className="max-w-screen-sm">
        <FormField label="Description">
          <Story />
        </FormField>
      </div>
    ),
  ],
} satisfies Meta<typeof Textarea>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Disabled: Story = { args: { disabled: true, value: 'Syncs orders every night.' } };
export const Error: Story = { args: { 'aria-invalid': true } };

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
      caption="Textarea states"
      columns={[...states]}
      rows={[
        {
          label: 'multi-line',
          cells: states.map((state) => (
            <div key={state} className="w-node-width">
              <Textarea rows={2} aria-label={`Textarea, ${state}`} placeholder="Placeholder" {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
