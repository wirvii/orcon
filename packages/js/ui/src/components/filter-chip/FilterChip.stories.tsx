// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Filter, FolderOpen } from 'lucide-react';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { FilterChip } from './FilterChip';
import { states } from './FilterChip.meta';

const meta = {
  title: 'Components/FilterChip',
  component: FilterChip,
  args: { label: 'Status', value: 'all', icon: Filter, onClick: fn() },
} satisfies Meta<typeof FilterChip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement, args }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: 'Status: all' }));
    await expect(args.onClick).toHaveBeenCalled();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Pressed: Story = { args: { pressed: true, value: 'Failed' } };
export const Menu: Story = { args: { menu: true, icon: FolderOpen, label: 'Project' } };
export const Disabled: Story = { args: { disabled: true } };

const stateProps: Record<(typeof states)[number], Record<string, unknown>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  pressed: { pressed: true, value: 'Failed' },
  menu: { menu: true },
  disabled: { disabled: true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="FilterChip states"
      columns={[...states]}
      rows={[{ label: 'chip', cells: states.map((state) => <FilterChip key={state} label="Status" value="all" icon={Filter} {...stateProps[state]} />) }]}
    />
  ),
};
