// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ScrollArea } from './ScrollArea';

const lines = Array.from({ length: 20 }, (_, index) => `Execution ${4800 + index}`);

const content = (
  <ul className="flex flex-col gap-1 p-2">
    {lines.map((line) => (
      <li key={line}>{line}</li>
    ))}
  </ul>
);

const meta = {
  title: 'Components/ScrollArea',
  component: ScrollArea,
  args: { label: 'Recent executions', className: 'h-node-width w-node-width bg-surface rounded-md border border-border', children: content },
} satisfies Meta<typeof ScrollArea>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { children: <p className="p-2">Short content that fits.</p> },
};

/** More content than fits: the region scrolls and takes keyboard focus. */
export const Overflow: Story = {
  play: async ({ canvasElement }) => {
    const region = within(canvasElement).getByRole('region', { name: 'Recent executions' });
    await expect(region.scrollHeight).toBeGreaterThan(region.clientHeight);
    region.focus();
    await expect(region).toHaveFocus();
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ScrollArea"
      columns={['default', 'overflow']}
      rows={[
        {
          label: 'area',
          cells: [
            <ScrollArea key="d" label="Short list" className="h-node-width w-node-width bg-surface rounded-md border border-border">
              <p className="p-2">Short content that fits.</p>
            </ScrollArea>,
            <ScrollArea key="o" label="Long list" className="h-node-width w-node-width bg-surface rounded-md border border-border">
              {content}
            </ScrollArea>,
          ],
        },
      ]}
    />
  ),
};
