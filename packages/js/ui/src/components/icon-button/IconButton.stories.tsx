// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Plus } from 'lucide-react';

import { MatrixTable } from '../../stories/MatrixTable';

import { IconButton } from './IconButton';
import { states } from './IconButton.meta';

const meta = {
  title: 'Components/IconButton',
  component: IconButton,
  args: { label: 'Add item', icon: Plus, variant: 'secondary' },
  argTypes: { variant: { control: 'inline-radio', options: ['primary', 'secondary', 'ghost', 'danger'] } },
} satisfies Meta<typeof IconButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Active: Story = { parameters: { pseudo: { active: true } } };
export const Disabled: Story = { args: { disabled: true } };
export const Loading: Story = { args: { loading: true } };

const variants = ['primary', 'secondary', 'ghost', 'danger'] as const;
const stateProps: Record<(typeof states)[number], { className?: string; disabled?: boolean; loading?: boolean }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  active: { className: 'pseudo-active' },
  disabled: { disabled: true },
  loading: { loading: true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="IconButton variants and states"
      columns={[...states]}
      rows={variants.map((variant) => ({
        label: variant,
        cells: states.map((state) => <IconButton key={state} label="Add item" icon={Plus} variant={variant} {...stateProps[state]} />),
      }))}
    />
  ),
};
