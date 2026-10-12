// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';

import { Sheet } from './Sheet';

const meta = {
  title: 'Components/Sheet',
  component: Sheet,
  args: {
    title: 'Node details',
    description: 'Status and item counts for the selected node.',
    children: <p>12 items succeeded, 1 item needs a decision.</p>,
  },
} satisfies Meta<typeof Sheet>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { trigger: <Button variant="secondary">Node details</Button> },
  play: async ({ canvasElement }) => {
    const trigger = within(canvasElement).getByRole('button', { name: 'Node details' });
    await userEvent.click(trigger);
    await expect(await within(document.body).findByRole('dialog', { name: 'Node details' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    await expect(trigger).toHaveFocus();
  },
};

/** Responsive placement: a bottom sheet on a phone viewport, a side panel from md up. */
export const Open: Story = { args: { defaultOpen: true } };
export const Bottom: Story = { args: { defaultOpen: true, side: 'bottom' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Sheet (the open states are their own stories)"
      columns={['default']}
      rows={[{ label: 'trigger', cells: [<Sheet key="d" title="Node details" trigger={<Button variant="secondary">Node details</Button>} />] }]}
    />
  ),
};
