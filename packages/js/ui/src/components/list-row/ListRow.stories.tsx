// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Workflow } from 'lucide-react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { ActiveToggle } from '../active-toggle/ActiveToggle';
import { StatusBadge } from '../status-badge/StatusBadge';

import { ListRow, type ListRowProps } from './ListRow';
import { states } from './ListRow.meta';

const meta = {
  title: 'Components/ListRow',
  component: ListRow,
  args: { href: '#flow', title: 'Sales agent', meta: 'Ventas WhatsApp · updated 1 hour ago' },
  decorators: [(Story) => <div className="border-border bg-surface w-full max-w-screen-sm rounded-lg border"><Story /></div>],
} satisfies Meta<typeof ListRow>;

export default meta;
type Story = StoryObj<typeof meta>;

const leading = <Workflow aria-hidden className="text-fg-muted size-icon-20" />;
const trailing = (
  <>
    <StatusBadge status="SUCCEEDED" />
    <ActiveToggle checked label="Sales agent" onCheckedChange={() => undefined} />
  </>
);

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The row is one link: its name is the title and nothing inside it is another link.
    await expect(within(canvasElement).getByRole('link', { name: /Sales agent/ })).toHaveAttribute('href', '#flow');
    await expect(within(canvasElement).getAllByRole('link')).toHaveLength(1);
  },
};
export const Hover: Story = { args: { className: 'bg-surface-hover' } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const WithTrailing: Story = { args: { leading, trailing } };
export const LongTitle: Story = {
  args: { title: 'Qualify every lead that arrives from the Meta Ads form and save it in the CRM with its score', meta: 'Ventas WhatsApp · updated yesterday' },
};

const stateProps: Record<(typeof states)[number], Partial<ListRowProps>> = {
  default: {},
  hover: { className: 'bg-surface-hover' },
  'focus-visible': {},
  'with-trailing': { leading, trailing },
  'long-title': { title: 'Qualify every lead that arrives from the Meta Ads form and save it in the CRM with its score' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ListRow states"
      columns={[...states]}
      rows={[
        {
          label: 'row',
          cells: states.map((state) => (
            <div key={state} className="border-border bg-surface w-panel-width rounded-lg border">
              <ListRow href="#flow" title="Sales agent" meta="Ventas WhatsApp" {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
