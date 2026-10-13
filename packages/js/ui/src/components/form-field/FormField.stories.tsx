// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Input } from '../input/Input';
import { RadioGroup, RadioGroupItem } from '../radio-group/RadioGroup';

import { FormField } from './FormField';
import { states } from './FormField.meta';

const meta = {
  title: 'Components/FormField',
  component: FormField,
  args: { label: 'Flow name', children: <Input placeholder="Untitled flow" /> },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="max-w-screen-sm">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof FormField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The control is named by the label.
    await expect(within(canvasElement).getByLabelText('Flow name')).toBeInTheDocument();
  },
};

export const Description: Story = {
  args: { description: 'Shown in the flow list and in notifications.' },
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByLabelText('Flow name');
    await expect(input).toHaveAccessibleDescription('Shown in the flow list and in notifications.');
  },
};

export const Required: Story = {
  args: { required: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('textbox', { name: /Flow name/ })).toBeRequired();
  },
};

export const Error: Story = {
  args: { error: 'Enter a name for the flow.', description: 'Shown in the flow list.' },
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByRole('textbox', { name: /Flow name/ });
    await expect(input).toBeInvalid();
    await expect(input).toHaveAccessibleDescription(/Enter a name for the flow/);
  },
};

export const Group: Story = {
  args: {
    group: true,
    label: 'Run mode',
    description: 'Applies to new executions.',
    children: (
      <RadioGroup defaultValue="queued">
        <RadioGroupItem value="queued" label="Queued" />
        <RadioGroupItem value="immediate" label="Immediate" />
      </RadioGroup>
    ),
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('radiogroup', { name: 'Run mode' })).toBeInTheDocument();
  },
};

const props: Record<(typeof states)[number], Partial<React.ComponentProps<typeof FormField>>> = {
  default: {},
  description: { description: 'Shown in the flow list.' },
  required: { required: true },
  error: { error: 'Enter a name for the flow.' },
  group: {
    group: true,
    children: (
      <RadioGroup defaultValue="a" aria-label="Choice">
        <RadioGroupItem value="a" label="Option A" />
        <RadioGroupItem value="b" label="Option B" />
      </RadioGroup>
    ),
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="FormField states"
      columns={[...states]}
      rows={[
        {
          label: 'field',
          cells: states.map((state) => (
            <div key={state} className="w-panel-width">
              <FormField label="Flow name" {...props[state]}>
                {props[state].children ?? <Input placeholder="Untitled flow" />}
              </FormField>
            </div>
          )),
        },
      ]}
    />
  ),
};
