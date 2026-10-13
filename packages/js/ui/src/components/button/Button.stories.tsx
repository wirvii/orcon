// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Button } from './Button';
import { states } from './Button.meta';

const meta = {
  title: 'Components/Button',
  component: Button,
  args: { children: 'Save changes', variant: 'primary' },
  argTypes: { variant: { control: 'inline-radio', options: ['primary', 'secondary', 'ghost', 'danger'] } },
} satisfies Meta<typeof Button>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Active: Story = { parameters: { pseudo: { active: true } } };
export const Disabled: Story = { args: { disabled: true } };
export const Loading: Story = { args: { loading: true } };

const variants = ['primary', 'secondary', 'ghost', 'danger'] as const;
const stateArgs: Record<(typeof states)[number], { className?: string; disabled?: boolean; loading?: boolean }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  active: { className: 'pseudo-active' },
  disabled: { disabled: true },
  loading: { loading: true },
};

/** Every variant in every state, for review and visual regression. */
export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Button variants and states"
      columns={[...states]}
      rows={variants.map((variant) => ({
        label: variant,
        cells: states.map((state) => (
          <Button key={state} variant={variant} {...stateArgs[state]}>
            Label
          </Button>
        )),
      }))}
    />
  ),
};
