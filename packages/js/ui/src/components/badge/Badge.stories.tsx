// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Badge } from './Badge';
import { states } from './Badge.meta';

const meta = {
  title: 'Components/Badge',
  component: Badge,
  args: { children: 'Draft' },
} satisfies Meta<typeof Badge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Active: Story = { args: { tone: 'active', children: 'Enabled' } };
export const Success: Story = { args: { tone: 'success', children: 'Verified' } };
export const Warning: Story = { args: { tone: 'warning', children: 'Deprecated' } };
export const Danger: Story = { args: { tone: 'danger', children: 'Blocked' } };
export const Attention: Story = { args: { tone: 'attention', children: 'Needs review' } };

const tones = { default: 'neutral', active: 'active', success: 'success', warning: 'warning', danger: 'danger', attention: 'attention' } as const;

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Badge tones"
      columns={[...states]}
      rows={[
        {
          label: 'badge',
          cells: states.map((state) => (
            <Badge key={state} tone={tones[state]}>
              {state}
            </Badge>
          )),
        },
      ]}
    />
  ),
};
